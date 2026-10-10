package single_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/brandsrx/supay/internal/adapters/siat/single"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

type serializerFunc func(context.Context, ports.FiscalDocument, int) (ports.SerializedDocument, error)

func (f serializerFunc) Serialize(c context.Context, d ports.FiscalDocument, e int) (ports.SerializedDocument, error) {
	return f(c, d, e)
}

type signerFunc func(context.Context, ports.FiscalSignRequest) (ports.FiscalSignResult, error)

func (f signerFunc) SignXML(c context.Context, r ports.FiscalSignRequest) (ports.FiscalSignResult, error) {
	return f(c, r)
}

type packerFunc func([]byte) (ports.PackedLot, error)

func (f packerFunc) PackDocument(xml []byte) (ports.PackedLot, error) { return f(xml) }
func (f packerFunc) PackLot(_ [][]byte) (ports.PackedLot, error) {
	panic("individual preparation must not pack a lot")
}

func TestPreparerOrdersStagesAndStopsAtFirstFailure(t *testing.T) {
	for _, failure := range []string{"", "validate", "sign", "pack"} {
		t.Run(failure, func(t *testing.T) {
			var stages []string
			sentinel := errors.New("stage failed")
			p := single.Preparer{
				Serializer: serializerFunc(func(_ context.Context, _ ports.FiscalDocument, _ int) (ports.SerializedDocument, error) {
					stages = append(stages, "validate")
					if failure == "validate" {
						return ports.SerializedDocument{}, sentinel
					}
					return ports.SerializedDocument{XML: []byte("<xml/>"), CUF: "cuf"}, nil
				}),
				Signer: signerFunc(func(_ context.Context, r ports.FiscalSignRequest) (ports.FiscalSignResult, error) {
					stages = append(stages, "sign")
					if r.Xml != "<xml/>" {
						t.Fatal("serializer output changed")
					}
					if failure == "sign" {
						return ports.FiscalSignResult{}, sentinel
					}
					return ports.FiscalSignResult{XmlFirmado: "<signed/>"}, nil
				}),
				Packer: packerFunc(func(xml []byte) (ports.PackedLot, error) {
					stages = append(stages, "pack")
					if string(xml) != "<signed/>" {
						t.Fatal("packer did not receive the signed bytes")
					}
					if failure == "pack" {
						return ports.PackedLot{}, sentinel
					}
					return ports.PackedLot{Archive: "archive", Hash: "hash"}, nil
				}),
			}
			result, err := p.Prepare(context.Background(), ports.FiscalDocument{Modalidad: fiscal.ModalidadElectronica}, fiscal.EmisionOnline)
			want := []string{"validate", "sign", "pack"}
			if failure != "" {
				want = want[:slices.Index(want, failure)+1]
				if !errors.Is(err, sentinel) {
					t.Fatalf("lost failure: %v", err)
				}
			} else if err != nil || result.Xml != "<signed/>" || result.Cuf != "cuf" || result.Archivo != "archive" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !slices.Equal(stages, want) {
				t.Fatalf("stages %v, want %v", stages, want)
			}
		})
	}
}

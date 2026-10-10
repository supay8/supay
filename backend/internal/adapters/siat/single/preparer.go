package single

import (
	"context"
	"fmt"

	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

// Preparer has no network transport: its only effects are the explicitly
// injected certificate signer. All intermediate bytes belong to this call.
type Preparer struct {
	Serializer ports.DocumentSerializer
	Signer     ports.FiscalSigner
	Packer     ports.LotPacker
}

func (p Preparer) Prepare(ctx context.Context, doc ports.FiscalDocument, emission int) (ports.FiscalResult, error) {
	serialized, err := p.Serializer.Serialize(ctx, doc, emission)
	if err != nil {
		return ports.FiscalResult{}, err
	}
	data := serialized.XML
	if doc.Modalidad == fiscal.ModalidadElectronica {
		if p.Signer == nil {
			return ports.FiscalResult{}, fmt.Errorf("firmador fiscal no configurado")
		}
		signed, err := p.Signer.SignXML(ctx, ports.FiscalSignRequest{Xml: string(data)})
		if err != nil {
			return ports.FiscalResult{}, err
		}
		data = []byte(signed.XmlFirmado)
	}
	packed, err := p.Packer.PackDocument(data)
	if err != nil {
		return ports.FiscalResult{}, err
	}
	return ports.FiscalResult{Cuf: serialized.CUF, Xml: string(data), Archivo: packed.Archive, XmlHash: packed.Hash}, nil
}

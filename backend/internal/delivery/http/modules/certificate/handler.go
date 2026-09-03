package certificate

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	uc *usecase.CertificateUsecase
}

func newHandler(uc *usecase.CertificateUsecase) *handler {
	return &handler{uc: uc}
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "id")
	if companyID == "" {
		companyID = r.URL.Query().Get("company_id")
	}
	// Solo multipart/form-data (no JSON base64) - p12_file binario + campos texto
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	file, header, err := r.FormFile("p12_file")
	if err != nil {
		deliveryHttp.RespondValidation(w, "p12_file es obligatorio (multipart/form-data, campo p12_file con .p12/.pfx)")
		return
	}
	defer file.Close()
	// Validar extensión
	lowerName := strings.ToLower(strings.TrimSpace(header.Filename))
	if !strings.HasSuffix(lowerName, ".p12") && !strings.HasSuffix(lowerName, ".pfx") {
		deliveryHttp.RespondValidation(w, "p12_file debe ser .p12 o .pfx")
		return
	}
	if header.Size > 5<<20 {
		deliveryHttp.RespondValidation(w, "p12_file excede 5MB")
		return
	}
	p12Bytes, err := io.ReadAll(file)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	if len(p12Bytes) == 0 {
		deliveryHttp.RespondValidation(w, "p12_file vacío")
		return
	}
	// Campos texto
	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		token = strings.TrimSpace(r.FormValue("token_delegado"))
	}
	in := usecase.CertificateInput{
		Name:        strings.TrimSpace(r.FormValue("name")),
		Type:        strings.TrimSpace(r.FormValue("type")),
		Token:       token,
		P12Password: strings.TrimSpace(r.FormValue("p12_password")),
		P12Bytes:    p12Bytes,
	}
	if v := strings.TrimSpace(r.FormValue("modalidad")); v != "" {
		if iv, err := strconv.Atoi(v); err == nil {
			in.Modalidad = &iv
		}
	}
	if v := strings.TrimSpace(r.FormValue("ambiente")); v != "" {
		uv := strings.ToUpper(v)
		in.Ambiente = &uv
	}
	cert, err := h.uc.Create(companyID, in)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusCreated, map[string]any{"data": cert})
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "id")
	certs, err := h.uc.List(companyID)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{"data": certs})
}

func (h *handler) getActive(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "id")
	cert, err := h.uc.GetActive(companyID)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{"data": cert})
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "certId")
	if err := h.uc.Delete(id); err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

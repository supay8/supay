package auth

import (
	"encoding/json"
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type Module struct {
	uc *usecase.AuthUsecase
}

func NewModule(uc *usecase.AuthUsecase) *Module { return &Module{uc: uc} }

func (m *Module) RegisterPublicRoutes(r chi.Router) {
	r.Post("/signup", m.signup)
	r.Post("/login", m.login)
}

func (m *Module) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/me", m.me)
	r.Get("/companies", m.listCompanies)
	r.Post("/companies", m.createCompany)
}

func (m *Module) signup(w http.ResponseWriter, r *http.Request) {
	var req usecase.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	result, err := m.uc.Signup(req)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusCreated, result)
}

func (m *Module) login(w http.ResponseWriter, r *http.Request) {
	var req usecase.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	result, err := m.uc.Login(req)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, result)
}

func (m *Module) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := deliveryHttp.UserIDFromContext(r.Context())
	if !ok {
		deliveryHttp.WriteErrorBody(w, http.StatusUnauthorized, deliveryHttp.CodeUnauthorized, "sesión no válida")
		return
	}
	user, err := m.uc.Me(userID)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, user)
}

func (m *Module) listCompanies(w http.ResponseWriter, r *http.Request) {
	userID, ok := deliveryHttp.UserIDFromContext(r.Context())
	if !ok {
		deliveryHttp.WriteErrorBody(w, http.StatusUnauthorized, deliveryHttp.CodeUnauthorized, "sesión no válida")
		return
	}
	companies, err := m.uc.ListCompanies(userID)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.RespondList(w, companies, len(companies), 0, 0)
}

func (m *Module) createCompany(w http.ResponseWriter, r *http.Request) {
	userID, ok := deliveryHttp.UserIDFromContext(r.Context())
	if !ok {
		deliveryHttp.WriteErrorBody(w, http.StatusUnauthorized, deliveryHttp.CodeUnauthorized, "sesión no válida")
		return
	}
	var req usecase.RegisterCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	company, err := m.uc.CreateCompany(userID, req)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusCreated, company)
}

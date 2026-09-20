package usecase

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AccessTokenIssuer interface {
	IssueAccessToken(userID, email string) (string, time.Time, error)
}

type AuthUsecase struct {
	repo      domain.AuthRepository
	companies *CompanyUsecase
	tokens    AccessTokenIssuer
	dummyHash []byte
}

func NewAuthUsecase(repo domain.AuthRepository, companies *CompanyUsecase, tokens AccessTokenIssuer) *AuthUsecase {
	dummyHash, _ := bcrypt.GenerateFromPassword([]byte("supay-invalid-password"), bcrypt.DefaultCost)
	return &AuthUsecase{repo: repo, companies: companies, tokens: tokens, dummyHash: dummyHash}
}

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResult struct {
	User        *domain.User `json:"user"`
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresAt   time.Time    `json:"expires_at"`
}

func (uc *AuthUsecase) Signup(req SignupRequest) (*AuthResult, error) {
	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if name == "" || len(name) > 150 {
		return nil, domain.NewBadRequestError("el nombre es obligatorio y debe tener hasta 150 caracteres")
	}
	if !validEmail(email) {
		return nil, domain.NewBadRequestError("el correo electrónico no es válido")
	}
	if err := validatePassword(req.Password); err != nil {
		return nil, err
	}
	if existing, err := uc.repo.GetUserByEmail(email); err == nil && existing != nil {
		return nil, domain.NewConflictError("ya existe un usuario registrado con este correo")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &domain.User{Name: name, Email: email, PasswordHash: string(hash), IsActive: true}
	if err := uc.repo.CreateUser(user); err != nil {
		if errors.Is(err, domain.ErrUserEmailConflict) {
			return nil, domain.NewConflictError(err.Error())
		}
		return nil, err
	}
	return uc.authResult(user)
}

func (uc *AuthUsecase) Login(req LoginRequest) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, err := uc.repo.GetUserByEmail(email)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(uc.dummyHash, []byte(req.Password))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewUnauthorizedError("correo o contraseña incorrectos")
		}
		return nil, err
	}
	if !user.IsActive || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, domain.NewUnauthorizedError("correo o contraseña incorrectos")
	}
	return uc.authResult(user)
}

func (uc *AuthUsecase) Me(userID string) (*domain.User, error) {
	user, err := uc.repo.GetUserByID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !user.IsActive) {
		return nil, domain.NewUnauthorizedError("la sesión ya no es válida")
	}
	return user, err
}

func (uc *AuthUsecase) ListCompanies(userID string) ([]domain.UserCompany, error) {
	if _, err := uc.Me(userID); err != nil {
		return nil, err
	}
	return uc.repo.ListCompanies(userID)
}

func (uc *AuthUsecase) CreateCompany(userID string, req RegisterCompanyRequest) (*domain.Company, error) {
	if _, err := uc.Me(userID); err != nil {
		return nil, err
	}
	company, err := uc.companies.prepareRegistration(req)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.CreateCompanyForUser(userID, company); err != nil {
		if errors.Is(err, domain.ErrCompanyNitConflict) {
			return nil, domain.NewConflictError(err.Error())
		}
		return nil, err
	}
	return company, nil
}

func (uc *AuthUsecase) authResult(user *domain.User) (*AuthResult, error) {
	token, expiresAt, err := uc.tokens.IssueAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: user, AccessToken: token, TokenType: "Bearer", ExpiresAt: expiresAt}, nil
}

func validEmail(value string) bool {
	if value == "" || len(value) > 320 {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(parsed.Address, value)
}

func validatePassword(value string) error {
	if len(value) < 8 {
		return domain.NewBadRequestError("la contraseña debe tener al menos 8 caracteres")
	}
	if len([]byte(value)) > 72 {
		return domain.NewBadRequestError("la contraseña debe tener hasta 72 bytes")
	}
	return nil
}

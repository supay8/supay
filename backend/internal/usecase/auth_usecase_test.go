package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type fakeAuthRepository struct {
	users       map[string]*domain.User
	memberships map[string][]domain.UserCompany
}

func newFakeAuthRepository() *fakeAuthRepository {
	return &fakeAuthRepository{users: map[string]*domain.User{}, memberships: map[string][]domain.UserCompany{}}
}

func (r *fakeAuthRepository) CreateUser(user *domain.User) error {
	if _, exists := r.users[user.Email]; exists {
		return domain.ErrUserEmailConflict
	}
	user.ID = "user-1"
	user.IsActive = true
	copy := *user
	r.users[user.Email] = &copy
	return nil
}

func (r *fakeAuthRepository) GetUserByEmail(email string) (*domain.User, error) {
	user, ok := r.users[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *user
	return &copy, nil
}

func (r *fakeAuthRepository) GetUserByID(id string) (*domain.User, error) {
	for _, user := range r.users {
		if user.ID == id {
			copy := *user
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeAuthRepository) HasCompanyAccess(userID, companyID string) (bool, error) {
	for _, membership := range r.memberships[userID] {
		if membership.Company.ID == companyID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeAuthRepository) ListCompanies(userID string) ([]domain.UserCompany, error) {
	return r.memberships[userID], nil
}

func (r *fakeAuthRepository) CreateCompanyForUser(userID string, company *domain.Company) error {
	company.ID = "company-1"
	r.memberships[userID] = append(r.memberships[userID], domain.UserCompany{Company: company, Role: domain.CompanyRoleOwner})
	return nil
}

type fakeAccessTokenIssuer struct{}

func (fakeAccessTokenIssuer) IssueAccessToken(userID, _ string) (string, time.Time, error) {
	if userID == "" {
		return "", time.Time{}, errors.New("missing user")
	}
	return "jwt-for-" + userID, time.Now().Add(time.Hour), nil
}

type authCompanyRepository struct{}

func (authCompanyRepository) Create(*domain.Company) error { return nil }
func (authCompanyRepository) GetByNit(string) (*domain.Company, error) {
	return nil, gorm.ErrRecordNotFound
}
func (authCompanyRepository) GetByID(string) (*domain.Company, error) {
	return nil, gorm.ErrRecordNotFound
}
func (authCompanyRepository) Update(*domain.Company) error { return nil }
func (authCompanyRepository) Delete(string) error          { return nil }

func TestAuthUsecaseSignupLoginAndMultipleCompanies(t *testing.T) {
	repo := newFakeAuthRepository()
	companies := NewCompanyUsecase(authCompanyRepository{}, nil, nil)
	uc := NewAuthUsecase(repo, companies, fakeAccessTokenIssuer{})

	signedUp, err := uc.Signup(SignupRequest{Name: " Ada ", Email: "ADA@example.com", Password: "correct-horse"})
	if err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if signedUp.User.Email != "ada@example.com" || signedUp.AccessToken != "jwt-for-user-1" || signedUp.User.PasswordHash == "" {
		t.Fatalf("signup inesperado: %+v", signedUp)
	}

	loggedIn, err := uc.Login(LoginRequest{Email: "ada@example.com", Password: "correct-horse"})
	if err != nil || loggedIn.User.ID != "user-1" {
		t.Fatalf("Login=%+v err=%v", loggedIn, err)
	}
	if _, err := uc.Login(LoginRequest{Email: "ada@example.com", Password: "incorrecta"}); err == nil {
		t.Fatal("se esperaba rechazo de contraseña incorrecta")
	}

	company, err := uc.CreateCompany("user-1", RegisterCompanyRequest{Nit: "123", BusinessName: "ACME"})
	if err != nil || company.ID != "company-1" {
		t.Fatalf("CreateCompany=%+v err=%v", company, err)
	}
	listed, err := uc.ListCompanies("user-1")
	if err != nil || len(listed) != 1 || listed[0].Role != domain.CompanyRoleOwner {
		t.Fatalf("ListCompanies=%+v err=%v", listed, err)
	}
}

func TestAuthUsecaseValidatesSignup(t *testing.T) {
	uc := NewAuthUsecase(newFakeAuthRepository(), NewCompanyUsecase(authCompanyRepository{}, nil, nil), fakeAccessTokenIssuer{})
	for _, req := range []SignupRequest{
		{},
		{Name: "Ada", Email: "bad", Password: "correct-horse"},
		{Name: "Ada", Email: "ada@example.com", Password: "short"},
	} {
		if _, err := uc.Signup(req); err == nil {
			t.Fatalf("se esperaba error para %+v", req)
		}
	}
}

package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/pariz/gountries"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/compose/types"
)

const (
	ValidateCodeAttempts = 10
	DefaultLocale        = types.Locale("es-MX")
)

type User struct {
	bun.BaseModel `bun:"table:users"`

	ID           int64        `bun:",pk,autoincrement" json:"id"`
	Name         string       `json:"name"`
	LastName     string       `json:"last_name"`
	Email        string       `json:"email"`
	PhoneNumber  string       `json:"phone_number"`
	Locale       types.Locale `json:"locale"`
	CodeAttempts int          `json:"-"`
	// Role         UserRole     `bun:"embed:role_" json:"role"`
}

func NewUser(name, lastName, email, phoneNumber, countryCode string) (*User, error) {
	const op = "user.NewUser"
	var locale types.Locale

	// Calculate locale
	country, err := gountries.New().FindCountryByAlpha(countryCode)
	if err != nil {
		locale = DefaultLocale
	} else {
		switch country.Alpha3 {
		case "USA":
			locale = types.Locale("en-" + country.Alpha2)
		case "MEX":
			locale = types.Locale("es-" + country.Alpha2)
		default:
			locale = DefaultLocale
		}
	}

	user := &User{
		Name:        name,
		LastName:    lastName,
		Email:       strings.ToLower(email),
		PhoneNumber: phoneNumber,
		Locale:      locale,
		// Role:        DefaultUserRole,
	}

	return user, nil
}

func (user *User) FullName() string {
	return user.Name + " " + user.LastName
}

func (user *User) HasEmail() bool {
	return user.Email != ""
}

func (user *User) HasPhoneNumber() bool {
	return user.PhoneNumber != ""
}

func (user *User) UpdateLocale(locale string) error {
	const op = "User.UpdateLocale"

	newLocale, err := types.NewLocaleString(locale)
	if err != nil {
		return ez.Wrap(op, err)
	}

	user.Locale = newLocale
	return nil
}

// Implement the Paginatable interface
func (u User) GetCursor() string {
	return fmt.Sprintf("%s:%d", u.GetSortValue(), u.GetUniqueValue())
}

func (u User) GetSortField() string {
	return `"user".last_name`
}

func (u User) GetSortValue() interface{} {
	lastName := u.LastName
	parts := strings.SplitN(lastName, " ", 2)

	if len(parts) > 0 {
		lastName = parts[0]
	}

	return lastName
}

func (u User) GetUniqueField() string {
	return `"user".id`
}

func (u User) GetUniqueValue() interface{} {
	return u.ID
}

// Implement the Model interface
func (u *User) Validate() error {
	// TODO: Add validation logic
	return nil
}

func (u *User) Insert(ctx context.Context, db bun.IDB) error {
	const op = "user.Insert"

	// Validate before inserting
	err := u.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewInsert().
		Model(u).
		Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

// Update updates an existing user
func (u *User) Update(ctx context.Context, db bun.IDB) error {
	const op = "user.Update"

	// Validate before updating
	err := u.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewUpdate().
		Model(u).
		WherePK().
		Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

// Delete removes a user from the database
func (u *User) Delete(ctx context.Context, db bun.IDB) error {
	const op = "user.Delete"

	_, err := db.NewDelete().
		Model(u).
		WherePK().
		Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

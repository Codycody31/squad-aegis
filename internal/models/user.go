package models

import (
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUsernameRequired = errors.New("username is required")
	ErrEmailRequired    = errors.New("email is required")
	ErrPasswordRequired = errors.New("password is required")
)

type User struct {
	Id         uuid.UUID `json:"id"`
	SteamId    int       `json:"steam_id"`
	Name       string    `json:"name"`
	Username   string    `json:"username"`
	Password   string    `json:"-"`
	SuperAdmin bool      `json:"super_admin"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (u *User) IsUsernameValid() error {
	const usernamePattern = `^[a-z0-9_]{1,32}$`

	// Compile the regex
	re := regexp.MustCompile(usernamePattern)

	// Check if the username matches the regex
	if !re.MatchString(u.Username) {
		return errors.New("username must be 1-32 characters long, all lowercase, and only contain a-z, 0-9, and _")
	}

	return nil
}

func (u *User) Validate() error {
	if err := u.IsUsernameValid(); err != nil {
		return err
	}
	if u.Password == "" {
		return ErrPasswordRequired
	}

	return nil
}

// SetPassword sets the password for the user using bcrypt
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

func (u *User) ComparePassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
}

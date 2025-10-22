package controller

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/vanclief/ez"
)

type I18n struct {
	Bundle *i18n.Bundle
}

func (t *I18n) String(locale Locale, key string) (string, error) {
	const op = "I18n.String"

	localizer := i18n.NewLocalizer(t.Bundle, strings.ToLower(locale.String()))

	str, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: key})
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	return str, nil
}

func (t *I18n) Template(locale Locale, key string, data map[string]interface{}) (string, error) {
	const op = "I18n.Template"

	localizer := i18n.NewLocalizer(t.Bundle, locale.String())

	str, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: data,
		PluralCount:  2,
	})
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	return str, nil
}

// Language represents supported languages
type Language string

const (
	English Language = "en"
	Spanish Language = "es"
)

const DEFAULT_LOCALE = "en-US"

func (l Language) IsValid() bool {
	switch l {
	case English, Spanish:
		return true
	default:
		return false
	}
}

// Locale is a string type with validation
type Locale string

// NewLocaleString creates a validated locale string
func NewLocaleString(locale string) (Locale, error) {
	const op = "NewLocaleString"

	locale = strings.TrimSpace(locale)
	if locale == "" {
		return Locale(DEFAULT_LOCALE), nil
	}

	// TODO: Currently I am avoiding returning errors and just returning a
	// locale, ideally I would need to implement the BCP 47 standard and return
	// whatever user locale they have and handle different locales in the app

	parts := strings.Split(locale, "-")
	if len(parts) < 1 {
		// If it does not have a standard format, return the default locale
		return Locale(DEFAULT_LOCALE), nil
	}

	lang := Language(strings.ToLower(parts[0]))
	if !lang.IsValid() {
		// If the language is unsupported, return the default locale
		return Locale(DEFAULT_LOCALE), nil
	}

	if len(locale) == 2 {
		switch Language(locale) {
		case English:
			return Locale("en-US"), nil
		case Spanish:
			return Locale("es-MX"), nil
		}
	}

	region := strings.ToUpper(parts[1])

	return Locale(fmt.Sprintf("%s-%s", lang, region)), nil
}

// Language returns the language part of the locale
func (l Locale) Language() Language {
	parts := strings.Split(string(l), "-")
	return Language(parts[0])
}

// Region returns the region part of the locale
func (l Locale) Region() string {
	parts := strings.Split(string(l), "-")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// String returns the locale string
func (l Locale) String() string {
	return string(l)
}

// MarshalJSON implements json.Marshaler
func (l Locale) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(l))
}

// UnmarshalJSON implements json.Unmarshaler
func (l *Locale) UnmarshalJSON(data []byte) error {
	const op = "Locale.UnmarshalJSON"

	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return ez.Wrap(op, err)
	}

	validated, err := NewLocaleString(str)
	if err != nil {
		return ez.Wrap(op, err)
	}

	*l = validated
	return nil
}

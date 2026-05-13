package validation

import (
	"reflect"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	instance *validator.Validate
	once     sync.Once
)

// Validator returns the package-level go-playground/validator singleton.
// Initialised on first call; safe for concurrent use thereafter.
func Validator() *validator.Validate {
	once.Do(func() {
		instance = validator.New()
		// Register tag name function so error messages report the JSON field
		// name rather than the Go struct field name.
		instance.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := fld.Tag.Get("json")
			if name == "" || name == "-" {
				return ""
			}
			// Strip options like ",omitempty"
			for i := 0; i < len(name); i++ {
				if name[i] == ',' {
					return name[:i]
				}
			}
			return name
		})
	})
	return instance
}

// Package validate provides functions to validate errors.
package validate

import (
	"log"
)

// Error will log fatal if the error is not nil.
func Error(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Panic will panic if the error is not nil.
func Panic(err error) {
	if err != nil {
		log.Panic(err)
	}
}

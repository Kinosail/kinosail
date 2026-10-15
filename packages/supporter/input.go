package supporter

import (
	"errors"
	"strings"
)

var (
	ErrInvalidKey             = errors.New("invalid supporter key")
	ErrInvalidRecognitionName = errors.New("invalid supporter recognition name")
)

func NewActivationInput(key, recognitionName string) (ActivationInput, error) {
	key = strings.TrimSpace(key)
	if !ValidKey(key) {
		return ActivationInput{}, ErrInvalidKey
	}
	input := ActivationInput{Key: key}
	if recognitionName == "" {
		return input, nil
	}
	if !ValidRecognitionName(recognitionName, true) {
		return ActivationInput{}, ErrInvalidRecognitionName
	}
	input.RecognitionName = &recognitionName
	return input, nil
}

func ParseActivationJSON(data []byte) (ActivationInput, error) {
	object, err := jsonObject(data)
	if err != nil || !exactFields(object, []string{"key"}, []string{"recognitionName"}) {
		return ActivationInput{}, ErrInvalid
	}
	key, ok := jsonString(object["key"])
	if !ok {
		return ActivationInput{}, ErrInvalid
	}
	input := ActivationInput{Key: key}
	if raw, found := object["recognitionName"]; found {
		name, nameOK := jsonString(raw)
		if !nameOK || !ValidRecognitionName(name, true) {
			return ActivationInput{}, ErrInvalid
		}
		input.RecognitionName = &name
	}
	return input, nil
}

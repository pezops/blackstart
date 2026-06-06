package crypto

import (
	"fmt"
	"strings"

	"github.com/pezops/blackstart"
)

// validateRequiredStaticPrivateKey validates a required static private key input when possible.
func validateRequiredStaticPrivateKey(op blackstart.Operation, key string) error {
	input, ok := op.Inputs[key]
	if !ok {
		return fmt.Errorf("missing required parameter: %s", key)
	}
	if !input.IsStatic() {
		return nil
	}
	value, err := blackstart.InputAs[string](input, true)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}
	if _, err = parsePrivateKeyPEM(value); err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}
	return nil
}

// validateRequiredStaticCSR validates a required static CSR input when possible.
func validateRequiredStaticCSR(op blackstart.Operation, key string) error {
	input, ok := op.Inputs[key]
	if !ok {
		return fmt.Errorf("missing required parameter: %s", key)
	}
	if !input.IsStatic() {
		return nil
	}
	value, err := blackstart.InputAs[string](input, true)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}
	if _, err = parseCSRPEM(value); err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}
	return nil
}

// validateRequiredStaticCertificate validates a required static certificate input when possible.
func validateRequiredStaticCertificate(op blackstart.Operation, key string) error {
	input, ok := op.Inputs[key]
	if !ok {
		return fmt.Errorf("missing required parameter: %s", key)
	}
	if !input.IsStatic() {
		return nil
	}
	value, err := blackstart.InputAs[string](input, true)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}
	if _, err = parseCertificatePEM(value); err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}
	return nil
}

// validateStaticCertificateChain validates an optional static certificate chain input.
func validateStaticCertificateChain(op blackstart.Operation, key string) error {
	input, ok := op.Inputs[key]
	if !ok || !input.IsStatic() {
		return nil
	}
	value, err := blackstart.InputAs[string](input, false)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if _, err = parseCertificateChainPEM(value); err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}
	return nil
}

// validateStaticProfile validates an optional static profile input.
func validateStaticProfile(op blackstart.Operation) error {
	input, ok := op.Inputs[inputProfile]
	if !ok || !input.IsStatic() {
		return nil
	}
	value, err := blackstart.InputAs[string](input, false)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputProfile, err)
	}
	if value == "" {
		return nil
	}
	if _, err = normalizeProfile(value); err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputProfile, err)
	}
	return nil
}

// validateStaticValidityHours validates an optional static validity_hours input.
func validateStaticValidityHours(op blackstart.Operation) error {
	input, ok := op.Inputs[inputValidityHours]
	if !ok || !input.IsStatic() {
		return nil
	}
	value, err := blackstart.InputAs[int64](input, false)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputValidityHours, err)
	}
	if value < 0 {
		return fmt.Errorf("parameter %s is invalid: validity_hours must be greater than zero", inputValidityHours)
	}
	return nil
}

// validateStaticSANs validates optional static SAN inputs when possible.
func validateStaticSANs(op blackstart.Operation) error {
	for _, key := range []string{inputIPAddresses, inputURIs} {
		input, ok := op.Inputs[key]
		if !ok || !input.IsStatic() {
			continue
		}
		values, err := blackstart.InputAs[[]string](input, false)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", key, err)
		}
		switch key {
		case inputIPAddresses:
			if _, err = parseIPAddresses(values); err != nil {
				return fmt.Errorf("parameter %s is invalid: %w", key, err)
			}
		case inputURIs:
			if _, err = parseURIs(values); err != nil {
				return fmt.Errorf("parameter %s is invalid: %w", key, err)
			}
		}
	}
	return nil
}

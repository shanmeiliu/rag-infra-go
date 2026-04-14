package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const recruiterUsernameAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func GenerateRecruiterUsername() (string, error) {
	suffix, err := randomString(5)
	if err != nil {
		return "", err
	}
	return "recruiter-" + suffix, nil
}

func randomString(n int) (string, error) {
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(recruiterUsernameAlphabet))))
		if err != nil {
			return "", fmt.Errorf("generate random string: %w", err)
		}
		out[i] = recruiterUsernameAlphabet[num.Int64()]
	}
	return string(out), nil
}

package gitea

import (
	"crypto/rand"
	"math/big"
	mrand "math/rand"
	"strings"
)

const (
	//password generation (https://golangbyexample.com/generate-random-password-golang/)
	lowerCharSet   = "abcdedfghijklmnopqrstuvwxyz"
	upperCharSet   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	specialCharSet = "!@#$%&*"
	numberSet      = "0123456789"
	allCharSet     = lowerCharSet + upperCharSet + specialCharSet + numberSet
)

// generatePassword returns a bespoke string that contains random characters with a given configuration to be used as a password
// original source from https://golangbyexample.com/generate-random-password-golang/
// tweaked to avoid gosec complaining about using math/rand.Intn() instead of crypto/rand.Int()
func generatePassword(passwordLength, minSpecialChar, minNum, minUpperCase int) (string, error) {
	var password strings.Builder

	//Set special character
	for i := 0; i < minSpecialChar; i++ {
		random, err := rand.Int(rand.Reader, big.NewInt(int64(len(specialCharSet))))
		if err != nil {
			return "", err
		}
		password.WriteString(string(specialCharSet[random.Int64()]))
	}

	//Set numeric
	for i := 0; i < minNum; i++ {
		random, err := rand.Int(rand.Reader, big.NewInt(int64(len(numberSet))))
		if err != nil {
			return "", err
		}
		password.WriteString(string(numberSet[random.Int64()]))
	}

	//Set uppercase
	for i := 0; i < minUpperCase; i++ {
		random, err := rand.Int(rand.Reader, big.NewInt(int64(len(upperCharSet))))
		if err != nil {
			return "", err
		}
		password.WriteString(string(upperCharSet[random.Int64()]))
	}

	remainingLength := passwordLength - minSpecialChar - minNum - minUpperCase
	for i := 0; i < remainingLength; i++ {
		random, err := rand.Int(rand.Reader, big.NewInt(int64(len(allCharSet))))
		if err != nil {
			return "", err
		}
		password.WriteString(string(allCharSet[random.Int64()]))
	}
	inRune := []rune(password.String())
	mrand.Shuffle(len(inRune), func(i, j int) {
		inRune[i], inRune[j] = inRune[j], inRune[i]
	})
	return string(inRune), nil
}

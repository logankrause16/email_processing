package catchall

import (
	"fmt"
	"math/rand"
	"strings"
)

const alphaRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Return a random string made up of characters passed
func randomRunes(prefix string, length int, runes ...string) string {
	chars := strings.Join(runes, "")
	var bytes = make([]byte, length)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = chars[b%byte(len(chars))]
	}
	return prefix + string(bytes)
}

// Return a random string of alpha characters
func randomAlpha(prefix string, length int) string {
	return randomRunes(prefix, length, alphaRunes)
}

// Given a list of strings, return one of the strings randomly
func randomItem(items ...string) string {
	var bytes = make([]byte, 1)
	rand.Read(bytes)
	return items[bytes[0]%byte(len(items))]
}

// Return a random domain name in the form "randomAlpha.net"
func randomDomainName() string {
	return fmt.Sprintf("%s.%s",
		randomAlpha("", 14),
		randomItem("net", "com", "org", "io", "gov"))
}

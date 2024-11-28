package utils

import "math/rand"

func RandCode() string {
	const randchars = "1234567890qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM"
	buf := make([]byte, 8)
	for i := range len(buf) {
		buf[i] = randchars[rand.Intn(len(randchars))]
	}
	return string(buf)
}

package fake

import "math/rand"

func JSON() map[string]any {
	return jsonRecursive(0, 5)
}

func jsonRecursive(depth, maxDepth int) map[string]any {
	if depth >= maxDepth {
		panic("max depth exceeded")
	}

	nkeys := 1 + rand.Intn(12)
	obj := make(map[string]any, nkeys)

	for i := 0; i < nkeys; i++ {
		key := String(1 + rand.Intn(32))
		if rand.Intn(100) < 70 || depth+1 >= maxDepth {
			obj[key] = String(1 + rand.Intn(32))
		} else {
			obj[key] = jsonRecursive(depth+1, maxDepth)
		}
	}

	return obj
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func String(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

package filematch

func jaroWinkler(s1, s2 string) float64 {
	r1 := []rune(s1)
	r2 := []rune(s2)

	if len(r1) == 0 || len(r2) == 0 {
		if len(r1) == 0 && len(r2) == 0 {
			return 1.0
		}
		return 0
	}

	maxDist := max(max(len(r1), len(r2))/2-1, 0)

	matches1 := make([]bool, len(r1))
	matches2 := make([]bool, len(r2))

	matches := 0
	for i := range r1 {
		start := max(0, i-maxDist)
		end := min(len(r2), i+maxDist+1)
		for j := start; j < end; j++ {
			if matches2[j] {
				continue
			}
			if r1[i] != r2[j] {
				continue
			}
			matches1[i] = true
			matches2[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0
	}

	transpositions := 0
	k := 0
	for i := range r1 {
		if !matches1[i] {
			continue
		}
		for !matches2[k] {
			k++
		}
		if r1[i] != r2[k] {
			transpositions++
		}
		k++
	}
	transpositions /= 2

	jaro := (float64(matches)/float64(len(r1)) +
		float64(matches)/float64(len(r2)) +
		float64(matches-transpositions)/float64(matches)) / 3.0

	prefix := 0
	maxPrefix := min(4, len(r1), len(r2))
	for i := 0; i < maxPrefix; i++ {
		if r1[i] == r2[i] {
			prefix++
		} else {
			break
		}
	}
	return jaro + float64(prefix)*0.1*(1-jaro)
}

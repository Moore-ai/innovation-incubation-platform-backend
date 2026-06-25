package filematch

func jaro(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	matchDist := max(len(s1), len(s2))/2 - 1
	if matchDist < 0 {
		matchDist = 0
	}
	matches := 0
	s1Matches := make([]bool, len(s1))
	s2Matches := make([]bool, len(s2))
	transpositions := 0
	for i := range s1 {
		lo := max(0, i-matchDist)
		hi := min(len(s2)-1, i+matchDist)
		for j := lo; j <= hi; j++ {
			if s2Matches[j] || s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0.0
	}
	k := 0
	for i := range s1 {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}
	transpositions /= 2
	return (float64(matches)/float64(len(s1)) +
		float64(matches)/float64(len(s2)) +
		float64(matches-transpositions)/float64(matches)) / 3.0
}

func jaroWinkler(s1, s2 string) float64 {
	j := jaro(s1, s2)
	prefix := 0
	for i := 0; i < min(4, min(len(s1), len(s2))); i++ {
		if s1[i] == s2[i] {
			prefix++
		} else {
			break
		}
	}
	return j + float64(prefix)*0.1*(1.0-j)
}

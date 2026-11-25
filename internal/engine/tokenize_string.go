package engine

// tokenize_string.c

type tokenized_string struct {
	count  int
	tokens []string
}

// C reads the terminating NUL (and one byte past it after an unterminated quote)
func tokenChar(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func Tokenize_String_Delimiter(str string, delimiter byte) *tokenized_string {
	var tokens []string

	p_start := 0
	p_end := p_start

	len_ := len(str)

	count := 0
	i := 0

	for tokenChar(str, p_start) != 0 && i < len_ {
		for tokenChar(str, p_start) == ' ' {
			p_start++
			i++
		}

		if tokenChar(str, p_start) == delimiter {
			p_start++
			i++
		}

		p_end = p_start
		for tokenChar(str, p_end) != delimiter && i < len_ {
			p_end++
			i++
		}

		tokens = append(tokens, str[p_start:p_end])

		p_start = p_end
		p_start++
		i++

		count++
	}

	return &tokenized_string{count: count, tokens: tokens}
}

func Tokenize_String(str string) *tokenized_string {
	var tokens []string
	var end_token byte

	p_start := 0
	p_end := p_start

	len_ := len(str)

	count := 0
	i := 0

	for tokenChar(str, p_start) != 0 && i < len_ {
		for tokenChar(str, p_start) == ' ' {
			p_start++
			i++
		}

		if tokenChar(str, p_start) == '"' {
			end_token = '"'
			p_start++
			i++
		} else {
			end_token = ' '
		}

		p_end = p_start
		for tokenChar(str, p_end) != end_token && i < len_ {
			p_end++
			i++
		}

		tokens = append(tokens, str[p_start:p_end])

		p_start = p_end

		if end_token == '"' {
			p_start++
			i++
		}

		count++
	}

	return &tokenized_string{count: count, tokens: tokens}
}

package commitsearch

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	errEmptyKey       = errors.New(`the query has a ":" with nothing before it`)
	errTextAfterQuote = errors.New(`the query has text right after a closing " quote`)
	errUnclosedQuote  = errors.New(`the query has an unclosed " quote`)
)

type tokenizer struct {
	maxLen int
	keys   []string
}

func (t tokenizer) Parse(q string) ([]Token, error) {
	if t.maxLen > 0 && utf8.RuneCountInString(q) > t.maxLen {
		return nil, fmt.Errorf("the search can not be longer than %d characters",
			t.maxLen)
	}
	s := scanner{state: scannerState_empty, keys: t.keys}
	for _, r := range q {
		if err := s.read(r); err != nil {
			return nil, err
		}
	}
	if err := s.end(); err != nil {
		return nil, err
	}
	return s.tokens, nil
}

type scannerState uint8

const (
	// between tokens
	scannerState_empty scannerState = iota
	// reading a token that did not open with a quote and has no key yet
	scannerState_readingText
	// reading a token that opened with a quote
	scannerState_readingQuotedText
	// reading the value of `key:`
	scannerState_readingValueOfKey
	// reading the value of `key:"`
	scannerState_readingQuotedValueOfKey
	// right after the quote that closed a token
	scannerState_quoteJustClosed
)

// Reads a query one rune at a time, collecting the tokens it holds.
type scanner struct {
	keys      []string
	tokens    []Token
	state     scannerState
	key       string
	value     strings.Builder
	isNegated bool
}

func (s *scanner) read(r rune) error {
	switch s.state {
	case scannerState_empty:
		return s.readEmpty(r)
	case scannerState_readingText:
		return s.readText(r)
	case scannerState_readingValueOfKey:
		return s.readValue(r)
	case scannerState_readingQuotedText, scannerState_readingQuotedValueOfKey:
		return s.readQuoted(r)
	case scannerState_quoteJustClosed:
		return s.readJustClosedQuote(r)
	}
	panic(fmt.Sprintf("unknown state %d", s.state))
}

// Ends the query, failing when a quoted token was left open.
func (s *scanner) end() error {
	switch s.state {
	case scannerState_empty, scannerState_quoteJustClosed:
		return nil
	case scannerState_readingText, scannerState_readingValueOfKey:
		return s.gotoState(scannerState_empty)
	case scannerState_readingQuotedText, scannerState_readingQuotedValueOfKey:
		return errUnclosedQuote
	}
	panic(fmt.Sprintf("unknown state %d", s.state))
}

// reads a rune for when the state is scannerState_empty
func (s *scanner) readEmpty(r rune) error {
	switch {
	case unicode.IsSpace(r):
		s.isNegated = false
	case r == '-' && !s.isNegated:
		// ignore repeated `-` if already negated.
		// the following `-` are parts of the token
		s.isNegated = true
	case r == '"':
		s.state = scannerState_readingQuotedText
	case r == ':':
		return errEmptyKey
	default:
		s.value.WriteRune(r)
		s.state = scannerState_readingText
	}
	return nil
}

// reads a rune for when the state is scannerState_readingText
func (s *scanner) readText(r rune) error {
	switch {
	case unicode.IsSpace(r):
		return s.gotoState(scannerState_empty)
	case r == ':':
		key := strings.ToLower(s.value.String())
		if !s.hasKey(key) {
			return fmt.Errorf("%q is not something that can be searched", key)
		}
		s.key = key
		s.value.Reset()
		s.state = scannerState_readingValueOfKey
	default:
		s.value.WriteRune(r)
	}
	return nil
}

// reads a rune for when the state is scannerState_readingValueOfKey
func (s *scanner) readValue(r rune) error {
	switch {
	case unicode.IsSpace(r):
		return s.gotoState(scannerState_empty)
	case r == '"' && s.value.Len() == 0:
		s.state = scannerState_readingQuotedValueOfKey
	default:
		s.value.WriteRune(r)
	}
	return nil
}

// reads a rune for when the state is scannerState_readingQuotedText/quotedValue
func (s *scanner) readQuoted(r rune) error {
	if r == '"' {
		return s.gotoState(scannerState_quoteJustClosed)
	}
	s.value.WriteRune(r)
	return nil
}

// reads a rune for when the state is scannerState_quoteJustClosed
func (s *scanner) readJustClosedQuote(r rune) error {
	if !unicode.IsSpace(r) {
		return errTextAfterQuote
	}
	s.state = scannerState_empty
	return nil
}

// Appends the token that was read, if any, and moves on to the next state.
func (s *scanner) gotoState(next scannerState) error {
	// Get current values
	key, value := s.key, s.value.String()
	isNegated := s.isNegated
	isQuoted := s.state == scannerState_readingQuotedText ||
		s.state == scannerState_readingQuotedValueOfKey
	// Reset the state
	s.key, s.isNegated, s.state = "", false, next
	s.value.Reset()
	if key != "" && value == "" {
		return fmt.Errorf("%q needs something after it", key)
	}
	if value == "" {
		return nil
	}
	// Append the new token
	s.tokens = append(s.tokens, Token{
		Key:       key,
		Value:     value,
		IsNegated: isNegated,
		IsQuoted:  isQuoted,
	})
	return nil
}

func (s *scanner) hasKey(key string) bool {
	return slices.ContainsFunc(s.keys, func(k string) bool {
		return strings.EqualFold(k, key)
	})
}

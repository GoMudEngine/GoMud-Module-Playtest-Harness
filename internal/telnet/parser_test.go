package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParserSplitsTextAndIAC(t *testing.T) {
	p := NewParser()
	// "hi" + IAC WILL GMCP + "yo"
	toks := p.Feed([]byte{'h', 'i', IAC, WILL, GMCP, 'y', 'o'})
	assert.Equal(t, []Token{
		{Kind: TokenText, Text: []byte("hi")},
		{Kind: TokenIAC, Command: WILL, Option: GMCP},
		{Kind: TokenText, Text: []byte("yo")},
	}, toks)
}

func TestParserExtractsGMCP(t *testing.T) {
	p := NewParser()
	payload := []byte(`Char.Vitals {"hp":10}`)
	in := append([]byte{IAC, SB, GMCP}, payload...)
	in = append(in, IAC, SE)
	toks := p.Feed(in)
	assert.Len(t, toks, 1)
	assert.Equal(t, TokenGMCP, toks[0].Kind)
	assert.Equal(t, "Char.Vitals", toks[0].GMCPPackage)
	assert.JSONEq(t, `{"hp":10}`, string(toks[0].GMCPData))
}

func TestParserHandlesSplitFeeds(t *testing.T) {
	p := NewParser()
	toks := p.Feed([]byte{IAC, SB, GMCP, 'R', 'o', 'o', 'm'})
	assert.Empty(t, toks) // incomplete SB, buffered
	toks = p.Feed(append([]byte(" {}"), IAC, SE))
	assert.Len(t, toks, 1)
	assert.Equal(t, "Room", toks[0].GMCPPackage)
}

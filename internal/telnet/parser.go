package telnet

import (
	"bytes"
	"strings"
)

// Telnet protocol bytes.
const (
	IAC  = 255
	DONT = 254
	DO   = 253
	WONT = 252
	WILL = 251
	SB   = 250
	SE   = 240
	GMCP = 201
)

// TokenKind classifies a parser output token.
type TokenKind int

const (
	TokenText TokenKind = iota
	TokenIAC            // a 3-byte IAC command (e.g. IAC WILL GMCP)
	TokenGMCP           // a complete GMCP sub-negotiation payload
)

// Token is one unit emitted by the parser.
type Token struct {
	Kind        TokenKind
	Text        []byte // TokenText
	Command     byte   // TokenIAC: WILL/WONT/DO/DONT
	Option      byte   // TokenIAC: the option byte (e.g. GMCP)
	GMCPPackage string // TokenGMCP: package name
	GMCPData    []byte // TokenGMCP: JSON payload (may be empty)
}

type parserState int

const (
	stData  parserState = iota
	stIAC               // saw IAC
	stCmd               // saw IAC + (WILL/WONT/DO/DONT), expecting option
	stSB                // inside SB ... awaiting IAC SE
	stSBIAC             // inside SB and saw IAC, expecting SE
)

// Parser is a stateful telnet stream decoder. Feed bytes; get tokens. Safe to
// call across arbitrary chunk boundaries (state + partial buffers persist).
type Parser struct {
	state   parserState
	cmd     byte
	sbBuf   []byte
	textBuf []byte
}

// NewParser returns a ready parser.
func NewParser() *Parser { return &Parser{state: stData} }

func (p *Parser) flushText(out []Token) []Token {
	if len(p.textBuf) > 0 {
		t := make([]byte, len(p.textBuf))
		copy(t, p.textBuf)
		out = append(out, Token{Kind: TokenText, Text: t})
		p.textBuf = p.textBuf[:0]
	}
	return out
}

// Feed consumes bytes and returns any complete tokens produced.
func (p *Parser) Feed(b []byte) []Token {
	var out []Token
	for _, c := range b {
		switch p.state {
		case stData:
			if c == IAC {
				out = p.flushText(out)
				p.state = stIAC
			} else {
				p.textBuf = append(p.textBuf, c)
			}
		case stIAC:
			switch c {
			case WILL, WONT, DO, DONT:
				p.cmd = c
				p.state = stCmd
			case SB:
				p.sbBuf = p.sbBuf[:0]
				p.state = stSB
			case IAC:
				p.textBuf = append(p.textBuf, IAC) // escaped 0xFF
				p.state = stData
			default:
				p.state = stData // ignore other 2-byte commands
			}
		case stCmd:
			out = append(out, Token{Kind: TokenIAC, Command: p.cmd, Option: c})
			p.state = stData
		case stSB:
			if c == IAC {
				p.state = stSBIAC
			} else {
				p.sbBuf = append(p.sbBuf, c)
			}
		case stSBIAC:
			if c == SE {
				// Only GMCP sub-negotiations become tokens; other options
				// (e.g. MSP, option byte != GMCP) are consumed and ignored.
				if len(p.sbBuf) > 0 && p.sbBuf[0] == GMCP {
					out = append(out, parseGMCP(p.sbBuf))
				}
				p.sbBuf = p.sbBuf[:0]
				p.state = stData
			} else {
				p.sbBuf = append(p.sbBuf, c) // IAC IAC inside SB -> literal
				p.state = stSB
			}
		}
	}
	out = p.flushText(out)
	return out
}

// parseGMCP splits a sub-negotiation buffer that begins with the GMCP option
// byte into a package name and JSON payload.
func parseGMCP(buf []byte) Token {
	body := buf
	if len(body) > 0 && body[0] == GMCP {
		body = body[1:]
	}
	s := string(body)
	pkg, data, _ := strings.Cut(s, " ")
	return Token{
		Kind:        TokenGMCP,
		GMCPPackage: strings.TrimSpace(pkg),
		GMCPData:    bytes.TrimSpace([]byte(data)),
	}
}

package tlsx

import (
	"encoding/binary"
	"errors"
	"io"
)

var ErrNotTLS = errors.New("not a tls client hello")

func PeekSNI(r io.Reader) (sni string, peeked []byte, err error) {
	hdr := make([]byte, 5)
	if _, err = io.ReadFull(r, hdr); err != nil {
		return "", hdr[:0], err
	}
	if hdr[0] != 0x16 {
		return "", hdr, ErrNotTLS
	}
	recLen := int(binary.BigEndian.Uint16(hdr[3:5]))
	if recLen < 4 || recLen > 1<<16-1 {
		return "", hdr, ErrNotTLS
	}
	body := make([]byte, recLen)
	if _, err = io.ReadFull(r, body); err != nil {
		return "", append(hdr, body[:0]...), err
	}
	peeked = append(hdr, body...)
	sni, err = sniFromHandshake(body)
	return sni, peeked, err
}

func sniFromHandshake(body []byte) (string, error) {
	if len(body) < 4 || body[0] != 0x01 {
		return "", ErrNotTLS
	}
	hsLen := int(body[1])<<16 | int(body[2])<<8 | int(body[3])
	if hsLen+4 > len(body) {
		hsLen = len(body) - 4
	}
	p := body[4 : 4+hsLen]
	if len(p) < 34 {
		return "", ErrNotTLS
	}
	p = p[34:] // version + random
	if len(p) < 1 {
		return "", ErrNotTLS
	}
	sidLen := int(p[0])
	p = p[1:]
	if len(p) < sidLen+2 {
		return "", ErrNotTLS
	}
	p = p[sidLen:]
	csLen := int(binary.BigEndian.Uint16(p[:2]))
	p = p[2:]
	if len(p) < csLen+1 {
		return "", ErrNotTLS
	}
	p = p[csLen:]
	compLen := int(p[0])
	p = p[1:]
	if len(p) < compLen {
		return "", ErrNotTLS
	}
	p = p[compLen:]
	if len(p) < 2 {
		return "", nil
	}
	extLen := int(binary.BigEndian.Uint16(p[:2]))
	p = p[2:]
	if len(p) < extLen {
		return "", ErrNotTLS
	}
	p = p[:extLen]
	for len(p) >= 4 {
		typ := binary.BigEndian.Uint16(p[:2])
		l := int(binary.BigEndian.Uint16(p[2:4]))
		p = p[4:]
		if len(p) < l {
			return "", ErrNotTLS
		}
		data := p[:l]
		p = p[l:]
		if typ != 0 {
			continue
		}
		if len(data) < 2 {
			return "", ErrNotTLS
		}
		list := data[2:]
		if len(list) < 3 {
			return "", ErrNotTLS
		}
		nameType := list[0]
		nl := int(binary.BigEndian.Uint16(list[1:3]))
		list = list[3:]
		if nameType != 0 || len(list) < nl {
			return "", ErrNotTLS
		}
		return string(list[:nl]), nil
	}
	return "", nil
}

package mlcrestorerdownloader

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func getCert(tmdData *bytes.Buffer, id int, numContents uint16) ([]byte, error) {
	certStart := 0x0B04 + 0x30*numContents
	certEnd := certStart + 0xA00

	if tmdData.Len() != int(certEnd-0x300) {
		certEnd -= 0x300
	}

	certSlice := tmdData.Bytes()[certStart:certEnd]

	switch id {
	case 0:
		return certSlice[:0x400], nil
	case 1:
		return certSlice[0x400:0x700], nil
	default:
		return nil, fmt.Errorf("invalid id: %d", id)
	}
}

func getDefaultCert() ([]byte, error) {
	resp, err := http.Get("http://ccs.cdn.c.shop.nintendowifi.net/ccs/download/000500101000400a/cetk") // OSv10
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download OSv10 cetk, status code: %d", resp.StatusCode)
	}

	cetkData := bytes.Buffer{}
	_, err = io.Copy(&cetkData, resp.Body)
	if err != nil {
		return nil, err
	}

	if cetkData.Len() >= 0x350+0x300 {
		return cetkData.Bytes()[0x350 : 0x350+0x300], nil
	}
	return nil, fmt.Errorf("failed to download OSv10 cetk, length: %d", cetkData.Len())
}

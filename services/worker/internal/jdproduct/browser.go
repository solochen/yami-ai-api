package jdproduct

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

var (
	ErrBrowserMissing  = errors.New("未找到本机浏览器，暂时无法读取京东商品页")
	ErrPageUnavailable = errors.New("该商品页暂时无法读取")
)

type cdpMessage struct {
	ID     int            `json:"id,omitempty"`
	Method string         `json:"method,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Result map[string]any `json:"result,omitempty"`
	Error  map[string]any `json:"error,omitempty"`
}

func Capture(ctx context.Context, pageURL string) ([]byte, []byte, error) {
	chrome := findChrome()
	if chrome == "" {
		return nil, nil, ErrBrowserMissing
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	port, err := freePort()
	if err != nil {
		return nil, nil, ErrPageUnavailable
	}
	profile, err := os.MkdirTemp("", "starai-jd-*")
	if err != nil {
		return nil, nil, ErrPageUnavailable
	}
	defer os.RemoveAll(profile)
	cmd := exec.CommandContext(ctx, chrome,
		"--headless=new",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-dev-shm-usage",
		"--no-sandbox",
		"--remote-allow-origins=*",
		"--remote-debugging-port="+port,
		"--user-data-dir="+profile,
		"about:blank",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, nil, ErrBrowserMissing
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	target, err := waitForPage(ctx, port)
	if err != nil {
		return nil, nil, ErrPageUnavailable
	}
	conn, err := dialCDP(ctx, target)
	if err != nil {
		return nil, nil, ErrPageUnavailable
	}
	defer conn.Close()
	return readProductAPIs(ctx, conn, pageURL)
}

func findChrome() string {
	if path := strings.TrimSpace(os.Getenv("CHROME_PATH")); path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func freePort() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	return port, err
}

func waitForPage(ctx context.Context, port string) (string, error) {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/json/list", nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				var targets []map[string]any
				decodeErr := json.NewDecoder(resp.Body).Decode(&targets)
				resp.Body.Close()
				if decodeErr == nil {
					for _, target := range targets {
						if stringValue(target["type"]) == "page" {
							if ws := stringValue(target["webSocketDebuggerUrl"]); ws != "" {
								return ws, nil
							}
						}
					}
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", ErrPageUnavailable
}

func dialCDP(ctx context.Context, wsURL string) (net.Conn, error) {
	parsed, err := url.Parse(wsURL)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", parsed.RequestURI(), parsed.Host, base64.StdEncoding.EncodeToString(key))
	if _, err := io.WriteString(conn, request); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(status, "101") {
		conn.Close()
		return nil, ErrPageUnavailable
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" {
			break
		}
	}
	return &bufferedConn{Conn: conn, reader: reader}, nil
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func readProductAPIs(ctx context.Context, conn net.Conn, pageURL string) ([]byte, []byte, error) {
	reader := bufio.NewReader(conn)
	nextID := 0
	call := func(method string, params map[string]any) int {
		nextID++
		body, _ := json.Marshal(cdpMessage{ID: nextID, Method: method, Params: params})
		_ = writeWS(conn, body)
		return nextID
	}
	call("Network.enable", nil)
	call("Page.enable", nil)
	call("Page.navigate", map[string]any{"url": pageURL})
	pending := map[int]string{}
	interesting := map[string]string{}
	bodies := map[string][]byte{}
	deadline := time.Now().Add(28 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetReadDeadline(deadline)
	for time.Now().Before(deadline) && (len(bodies["ware"]) == 0 || len(bodies["graphic"]) == 0) {
		payload, err := readWS(reader, conn)
		if err != nil {
			break
		}
		var msg cdpMessage
		if json.Unmarshal(payload, &msg) != nil {
			continue
		}
		if name, ok := pending[msg.ID]; ok {
			delete(pending, msg.ID)
			if body := responseBody(msg.Result); len(body) > len(bodies[name]) {
				bodies[name] = body
			}
			continue
		}
		if msg.Method == "Network.responseReceived" || msg.Method == "Network.requestWillBeSent" {
			response, _ := msg.Params["response"].(map[string]any)
			request, _ := msg.Params["request"].(map[string]any)
			rawURL := firstNonEmpty(stringValue(response["url"]), stringValue(request["url"]))
			postData := stringValue(request["postData"])
			haystack := rawURL + "\n" + postData
			name := ""
			switch {
			case strings.Contains(haystack, "pc_detailpage_wareBusiness"):
				name = "ware"
			case strings.Contains(haystack, "pc_item_getWareGraphic"):
				name = "graphic"
			}
			if name != "" {
				interesting[stringValue(msg.Params["requestId"])] = name
			}
		}
		if msg.Method == "Network.loadingFinished" {
			requestID := stringValue(msg.Params["requestId"])
			if name := interesting[requestID]; name != "" {
				pending[call("Network.getResponseBody", map[string]any{"requestId": requestID})] = name
			}
		}
	}
	ware := usefulBody(bodies["ware"])
	graphic := usefulBody(bodies["graphic"])
	if len(ware) == 0 && len(graphic) == 0 {
		return nil, nil, ErrPageUnavailable
	}
	return ware, graphic, nil
}

func usefulBody(body []byte) []byte {
	text := strings.ToLower(string(body))
	if text == "" || strings.Contains(text, "no access") || strings.Contains(text, "京东验证") || strings.Contains(text, "risk_handler") {
		return nil
	}
	return body
}

func responseBody(result map[string]any) []byte {
	if result == nil {
		return nil
	}
	body := stringValue(result["body"])
	if body == "" {
		return nil
	}
	if encoded, _ := result["base64Encoded"].(bool); encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return nil
		}
		return decoded
	}
	return []byte(body)
}

func writeWS(conn net.Conn, payload []byte) error {
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	header := []byte{0x81}
	n := len(payload)
	switch {
	case n < 126:
		header = append(header, byte(0x80|n))
	case n < 65536:
		header = append(header, 0xFE, byte(n>>8), byte(n))
	default:
		header = append(header, 0xFF)
		for shift := 56; shift >= 0; shift -= 8 {
			header = append(header, byte(n>>shift))
		}
	}
	frame := append(header, mask...)
	masked := make([]byte, n)
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	_, err := conn.Write(append(frame, masked...))
	return err
}

func readWS(r *bufio.Reader, conn net.Conn) ([]byte, error) {
	var message []byte
	for {
		header, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		second, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		fin := header&0x80 != 0
		opcode := header & 0x0F
		length := int(second & 0x7F)
		switch length {
		case 126:
			buf := make([]byte, 2)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			length = int(buf[0])<<8 | int(buf[1])
		case 127:
			buf := make([]byte, 8)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			length = 0
			for _, b := range buf {
				length = length<<8 | int(b)
			}
		}
		if second&0x80 != 0 {
			if _, err := io.CopyN(io.Discard, r, 4); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
		switch opcode {
		case 0x8:
			return nil, io.EOF
		case 0x9:
			_ = writeControl(conn, 0x8A, payload)
			continue
		case 0xA:
			continue
		}
		message = append(message, payload...)
		if fin {
			return message, nil
		}
	}
}

func writeControl(conn net.Conn, opcode byte, payload []byte) error {
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	frame := []byte{opcode, byte(0x80 | len(payload))}
	frame = append(frame, mask...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	_, err := conn.Write(frame)
	return err
}

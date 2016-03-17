package main

import (
  "log"
  "fmt"
  "net/url"
  "net/http"
  "io/ioutil"
  "encoding/json"
  "time"
  "os"
  "os/signal"
  "flag"
  "golang.org/x/net/websocket"
)

type NegotiationParams struct {
  Url string `json:"Url"`
  ConnectionToken string
  ConnectionId string
  KeepAliveTimeout float32
  DisconnectTimeout float32
  ConnectionTimeout float32
  TryWebSockets bool
  ProtocolVersion string
  TransportConnectTimeout float32
  LogPollDelay float32
}

var scheme, addr, origin, hubname string

var connectionProtocol = "webSockets" // currently no support for server sent events

func main() {
  log.Println("Starting SignalR Connection")

  flag.StringVar(&scheme, "scheme", "https", "protocol for connecting - http(s)")
  flag.StringVar(&addr, "addr", "", "endpoint to connect to")
  flag.StringVar(&hubname, "hubname", "", "SignalR hub to connect to")

  flag.Parse()

  origin = fmt.Sprintf("%s://%s", scheme, addr)

  interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

  params := Negotiate(addr)

  cli := &http.Client{}

  connectionData := "[%7B%22Name%22:%22" + hubname + "%22%7D]"

  startUrl := BuildStartUrl(params.ProtocolVersion, connectionProtocol, connectionData, params.ConnectionToken)

  resp, err := cli.Get(startUrl.String())
  if err != nil {
    panic(err)
  }
  defer resp.Body.Close()

  body, _ := ioutil.ReadAll(resp.Body)

  log.Println(string(body))

  connectUrl := BuildConnectUrl(params.ProtocolVersion, connectionProtocol, connectionData, params.ConnectionToken)

  ws, err := websocket.Dial(connectUrl.String(), "", origin)
  if err != nil {
    panic(err)
  }

  defer ws.Close()

  done := make(chan struct{})

  var msgSize = make([]byte, 512)

  ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

  go func() {
		defer ws.Close()
		defer close(done)
		for {
			n, err := ws.Read(msgSize)
      if err != nil {
        log.Println("read:", err)
				return
      }
			log.Printf("Received: %s.\n", msgSize[:n])
		}
	}()

  for {
		select {
		case <-ticker.C:
			msg, err := ws.Write([]byte("data="))
			if err != nil {
				log.Println("write:", err)
				return
			}
      log.Println("Msg: ", msg)
		case <-interrupt:
			log.Println("interrupt")
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			_, err := ws.Write([]byte("close"))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			ws.Close()
			return
		}
	}

}

func Negotiate(addr string) (params NegotiationParams) {
  params = NegotiationParams{}

  u := url.URL{Scheme: scheme, Host: addr, Path: "/signalr/negotiate"}

  client := &http.Client{}

  resp, err := client.Get(u.String())
  if err != nil {
    panic(err)
  }
  defer resp.Body.Close()

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    log.Println(err)
    return
  }

  err = json.Unmarshal(body, &params)
  if err != nil {
    log.Println(err)
  }

  return
}

func BuildSendUrl(protocol, transport, connectionData, connectionToken string) url.URL {
  u := url.URL{Scheme: "wss", Host: addr, Path: "/signalr/send"}
  query := u.Query()
  query = AppendCommonParameters(query, protocol, transport, connectionData, connectionToken)
  u.RawQuery = query.Encode()
  u.RawQuery += "&connectionData=" + connectionData

  return u
}

func BuildStartUrl(protocol, transport, connectionData, connectionToken string) url.URL {
  u := url.URL{Scheme: scheme, Host: addr, Path: "/signalr/start"}
  query := u.Query()
  query = AppendCommonParameters(query, protocol, transport, connectionData, connectionToken)
  u.RawQuery = query.Encode()
  u.RawQuery = query.Encode()
  u.RawQuery += "&connectionData=" + connectionData

  return u
}

func BuildConnectUrl(protocol, transport, connectionData, connectionToken string) url.URL {
  u := url.URL{Scheme: "wss", Host: addr, Path: "/signalr/connect"}
  query := u.Query()
  query = AppendCommonParameters(query, protocol, transport, connectionData, connectionToken)
  u.RawQuery = query.Encode()
  u.RawQuery = query.Encode()
  u.RawQuery += "&connectionData=" + connectionData

  return u
}

func AppendCommonParameters(query url.Values,
  protocol, transport, connectionData, connectionToken string) url.Values {
  query = AppendProtocol(query, protocol)
  query = AppendTransport(query, transport)
  query = AppendConnectionToken(query, connectionToken)

  return query
}

func AppendProtocol(query url.Values, protocol string) url.Values {
  query.Set("clientProtocol", protocol)
  return query
}

func AppendTransport(query url.Values, transport string) url.Values {
  query.Set("transport", transport)
  return query
}

func AppendConnectionData(query url.Values, connectionData string) url.Values {
  query.Set("connectionData", connectionData)
  return query
}

func AppendConnectionToken(query url.Values, connectionToken string) url.Values {
  query.Set("connectionToken", connectionToken)
  return query
}

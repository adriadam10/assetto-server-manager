package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/kahlys/proxy"
)

var (
	proxyAddress  string
	proxyTCPPort  int
	proxyHTTPPort int
	proxyUDPPort  int
)

func init() {
	flag.StringVar(&proxyAddress, "addr", "", "proxy address")
	flag.IntVar(&proxyTCPPort, "tcp", 0, "proxy TCP port")
	flag.IntVar(&proxyUDPPort, "udp", 0, "proxy UDP port")
	flag.IntVar(&proxyHTTPPort, "http", 0, "proxy HTTP port")
	flag.Parse()
}

func main() {
	err := proxyACServerUDP()

	if err != nil {
		panic(err)
	}

	go func() {
		err = proxyACServerTCP()

		if err != nil {
			panic(err)
		}
	}()

	err = proxyACServerHTTP() // blocking

	if err != nil {
		panic(err)
	}
}

func proxyACServerTCP() error {
	localAddr := fmt.Sprintf("0.0.0.0:%d", proxyTCPPort)
	remoteAddr := fmt.Sprintf("%s:%d", proxyAddress, proxyTCPPort)

	s := proxy.Server{
		Addr:   localAddr,
		Target: remoteAddr,
		ModifyRequest: func(b *[]byte) {
			fmt.Println("TCP Client sent to server: ")
			printHex(*b)
		},
		ModifyResponse: func(b *[]byte) {
			fmt.Println("TCP Server sent to client: ")
			printHex(*b)
		},
	}

	log.Printf("Proxying TCP: %s -> %s\n", localAddr, remoteAddr)

	return s.ListenAndServe()
}

var forwardingUDPAddr *net.UDPAddr

func proxyACServerUDP() error {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", proxyUDPPort))

	if err != nil {
		return err
	}

	udpProxyListener, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		return err
	}

	remoteAddr := fmt.Sprintf("%s:%d", proxyAddress, proxyUDPPort)

	udpProxyForwarder, err := net.Dial("udp", remoteAddr)

	if err != nil {
		return err
	}

	log.Printf("Proxying UDP: %s -> %s\n", udpAddr.String(), remoteAddr)

	go func() {
		for {
			buf := make([]byte, 1024)

			n, addr, err := udpProxyListener.ReadFromUDP(buf)

			if err != nil {
				log.Println("could not read from udp proxy listener")
				log.Println(err)
				break
			}

			forwardingUDPAddr = addr

			_, err = udpProxyForwarder.Write(buf[:n])

			if err != nil {
				log.Println("could not write to udp proxy forwarder")
				log.Println(err)
				break
			}

			//fmt.Println("Sent UDP Message", n)
			//printHex(buf[:n])

			n, err = udpProxyForwarder.Read(buf)

			if err != nil {
				log.Println("could not read from udp proxy forwarder")
				log.Println(err)
				break
			}

			_, err = udpProxyListener.WriteToUDP(buf[:n], addr)

			if err != nil {
				log.Println("could not write back to udp proxy listener")
				log.Println(err)
				break
			}

			//fmt.Println("Received UDP Message", n)
			//printHex(buf[:n])
		}
	}()

	go func() {
		for {
			buf := make([]byte, 1024)

			n, err := udpProxyForwarder.Read(buf)

			if err != nil {
				log.Println("could not read from udp proxy listener")
				log.Println(err)
				break
			}

			_, err = udpProxyListener.WriteToUDP(buf[:n], forwardingUDPAddr)

			if err != nil {
				log.Println("could not write to udp proxy forwarder")
				log.Println(err)
				break
			}

			//fmt.Println("Sent UDP Message", n)
			//printHex(buf[:n])

			n, err = udpProxyListener.Read(buf)

			if err != nil {
				log.Println("could not read from udp proxy forwarder")
				log.Println(err)
				break
			}

			_, err = udpProxyForwarder.Write(buf[:n])

			if err != nil {
				log.Println("could not write back to udp proxy listener")
				log.Println(err)
				break
			}

			//fmt.Println("Received UDP Message", n)
			//printHex(buf[:n])
		}
	}()

	return nil
}

func proxyACServerHTTP() error {
	targetURL, err := url.Parse(fmt.Sprintf("http://%s:%d", proxyAddress, proxyHTTPPort))

	if err != nil {
		return err
	}

	r := httputil.NewSingleHostReverseProxy(targetURL)

	r.ModifyResponse = func(r *http.Response) error {
		fmt.Println("Got HTTP Response for URL", r.Request.URL.String())

		buf, err := ioutil.ReadAll(r.Body)

		if err != nil {
			return err
		}

		defer r.Body.Close()

		//fmt.Println(string(buf))

		r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))

		return nil
	}

	localURL := fmt.Sprintf("0.0.0.0:%d", proxyHTTPPort)

	log.Printf("Proxying HTTP: %s -> %s\n", localURL, targetURL.String())

	return http.ListenAndServe(localURL, r)
}

func printHex(b []byte) {
	fmt.Printf("hex: %x\n\n", b)
}

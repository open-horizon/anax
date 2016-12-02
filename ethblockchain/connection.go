package ethblockchain

import (
    "github.com/golang/glog"
    "net/url"
    "regexp"
    "strconv"
    "strings"
)

type RPC_Connection struct {
    host                  string
    port                  int
    fullURL               string
}

func RPC_Connection_Factory(host string, port int, fullURL string) *RPC_Connection {
    if len(fullURL) != 0 {
        if parsed_url,err := url.Parse(fullURL); err != nil {
            glog.Errorf("Input URL %v is not parseable: %v", fullURL, err)
            return nil
        } else if len(parsed_url.Host) == 0 || len(parsed_url.Scheme) == 0 {
            glog.Errorf("Input URL %v is not parseable, host or scheme is empty", fullURL)
            return nil
        } else if parsed_url.Scheme != "http" {
            glog.Errorf("Input URL %v is not parseable, scheme is not http", fullURL)
            return nil
        } else {

            err := error(nil)
            parsed_host := parsed_url.Host
            parsed_port := 0
            if strings.Contains(parsed_host, ":") {
                parsed_host = parsed_url.Host[:strings.Index(parsed_url.Host,":")]
                if parsed_port,err = strconv.Atoi(parsed_url.Host[strings.Index(parsed_url.Host,":")+1:]); err != nil {
                    glog.Errorf("Unable to convert port in input URL %v to an integer: %v", fullURL, err)
                    return nil
                }
            }

            if !validHostName(parsed_host) {
                glog.Errorf("Input URL %v is not parseable, host name is not valid", fullURL)
                return nil
            }
            return &RPC_Connection{
                        host: parsed_host,
                        port: parsed_port,
                        fullURL: fullURL,
                    }
        }

    } else if len(host) == 0 || port == 0 {
        glog.Errorf("Input host %v is empty or port %v is zero.", host, port)
        return nil
    } else if !validHostName(host) {
        glog.Errorf("Input host %v is not parseable, host name is not valid", host)
        return nil
    } else {
        construct_URL := "http://"+host+":"+strconv.Itoa(port)
        return &RPC_Connection{
                        host: host,
                        port: port,
                        fullURL: construct_URL,
                    }
    }
}

func (self *RPC_Connection) Get_fullURL() string {
    return self.fullURL
}


// ================ Utility functions =======================================================

func validHostName(host string) bool {
    host_regex := `^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`
    if rx,err := regexp.Compile(host_regex); err != nil {
        glog.Errorf("Unable to compile hostname regex: %v", err)
        return false
    } else {
        return rx.Match([]byte(host))
    }
}


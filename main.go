package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var errFetchingURL = errors.New("error fetching url")

type configs struct {
	Job []struct {
		URL    string `mapstructure:"url"`
		Count  int    `mapstructure:"count"`
		Method string `mapstructure:"method"`
		Host   string `mapstructure:"host"`
	}
	Wait time.Duration
}

type result struct {
	Message string
	Err     error
}

func main() {
	viper.SetConfigName("urls")
	viper.SetConfigType("hcl")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msgf("fatal error config file")
	}

	configs := &configs{}

	err = viper.Unmarshal(configs)
	if err != nil {
		log.Fatal().Err(err).Msgf("fatal error config file")
	}

	log.Info().Msgf("CONFIG %+v", configs)

	ch := make(chan result)

	var wg sync.WaitGroup

	wg.Add(len(configs.Job))

	processURLs(configs, &wg, ch)

	go func() {
		for res := range ch {
			if res.Err != nil {
				log.Info().Msgf("%s", res.Err.Error())
			} else {
				log.Info().Msgf("%s", res.Message)
			}
		}
	}()

	wg.Wait()
}

func processURLs(configs *configs, wg *sync.WaitGroup, ch chan result) {
	for i := range configs.Job {
		go func(i int) {
			log.Info().Msgf("%d - %s", configs.Job[i].Count, configs.Job[i].URL)

			defer wg.Done()

			n := 0

			for n <= configs.Job[i].Count {
				res, err := fetchURL(configs.Job[i].URL, configs.Job[i].Method, configs.Job[i].Host)
				if err != nil {
					ch <- result{Message: "", Err: fmt.Errorf("%d - %w", n, err)}

					n++

					continue
				}

				ch <- result{Message: fmt.Sprintf("%d - %s", n, res), Err: nil}

				time.Sleep(configs.Wait)

				n++
			}
		}(i)
	}
}

func fetchURL(addr, method, host string) (string, error) {
	//nolint: exhaustivestruct, gosec
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	//nolint: exhaustivestruct
	client := &http.Client{Transport: tr}

	url, err := url.Parse(addr)
	if err != nil {
		return "", fmt.Errorf("%s - %w", err.Error(), errFetchingURL)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url.String(), nil)
	if err != nil {
		return "", fmt.Errorf("%s - %w", err.Error(), errFetchingURL)
	}

	req.Host = host

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s - %w", err.Error(), errFetchingURL)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatal().Err(err).Msg("error defering body close")
		}
	}()

	var bodyBytes []byte

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("%s - %w", err.Error(), errFetchingURL)
		}
	}

	return fmt.Sprintf("%d - %s", resp.StatusCode, string(bodyBytes)), nil
}

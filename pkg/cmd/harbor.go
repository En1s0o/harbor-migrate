package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"os/exec"
	"strconv"
)

type HarborConfig struct {
	URL      string
	Username string
	Password string
}

func transferProjects(ctx context.Context, source HarborConfig, target HarborConfig) error {
	// 获取 source 所有 Project，并且在 target 创建对应的 Project
	return fetchPage(ctx, source, "/api/v2.0/projects?with_detail=true", func(data []byte) error {
		var projects []struct {
			Metadata struct {
				Public string `json:"public"`
			} `json:"metadata"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &projects); err != nil {
			return err
		} else if len(projects) == 0 {
			return io.EOF
		}

		for _, project := range projects {
			body := fmt.Sprintf(`{"project_name":"%s","metadata":{"public":"%s"},"storage_limit":-1,"registry_id":null}`, project.Name, project.Metadata.Public)
			if out, err := post(ctx, target, "/api/v2.0/projects", []byte(body)); err != nil {
				return err
			} else if len(out) > 0 {
				logger.Infof("%s", out)
			}
		}

		return nil
	})
}

func transferImages(ctx context.Context, source HarborConfig, target HarborConfig) {
	sourceURL, err := urlpkg.Parse(source.URL)
	if err != nil {
		panic(err)
	}

	targetURL, err := urlpkg.Parse(target.URL)
	if err != nil {
		panic(err)
	}

	// Login
	sourceLoginCmd := fmt.Sprintf("docker login -u %s -p %s %s", source.Username, source.Password, sourceURL.Host)
	targetLoginCmd := fmt.Sprintf("docker login -u %s -p %s %s", target.Username, target.Password, targetURL.Host)
	runCmd(ctx, sourceLoginCmd)
	runCmd(ctx, targetLoginCmd)

	// Pull & Tag & Push
	imageNames, err := fetchImageNames(ctx, source)
	if err != nil {
		panic(err)
	}
	for _, imageName := range imageNames {
		sourceImageName := fmt.Sprintf("%s/%s", sourceURL.Host, imageName)
		targetImageName := fmt.Sprintf("%s/%s", targetURL.Host, imageName)
		pullCmd := fmt.Sprintf("docker pull %s -a", sourceImageName)
		tagCmd := fmt.Sprintf("docker images %s --format '{{.Tag}}' | grep -v '<none>' | xargs -I {} docker tag %s:{} %s:{}", sourceImageName, sourceImageName, targetImageName)
		pushCmd := fmt.Sprintf("docker push %s -a", targetImageName)
		// docker images pcr.io/um-ems/ems-operations-center-server --format '{{.Tag}}'
		fmt.Printf("transfer %s => %s\n", sourceImageName, targetImageName)
		runCmd(ctx, pullCmd)
		runCmd(ctx, tagCmd)
		runCmd(ctx, pushCmd)
	}
}

func runCmd(ctx context.Context, command string) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	if err := cmd.Wait(); err != nil {
		panic(err)
	}
}

func fetchImageNames(ctx context.Context, harbor HarborConfig) ([]string, error) {
	var imageNames []string
	err := fetchPage(ctx, harbor, "/api/v2.0/repositories", func(data []byte) error {
		if len(data) == 0 {
			return io.EOF
		}

		var repositories []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &repositories); err != nil {
			return err
		} else if len(repositories) == 0 {
			return io.EOF
		}

		for _, repository := range repositories {
			imageNames = append(imageNames, repository.Name)
		}

		return nil
	})

	return imageNames, err
}

func post(ctx context.Context, harbor HarborConfig, ref string, body []byte) ([]byte, error) {
	u, err := urlpkg.Parse(harbor.URL)
	if err != nil {
		return nil, err
	}

	u, err = u.Parse(ref)
	if err != nil {
		return nil, err
	}

	req, err := makeHTTPRequest(ctx, http.MethodPost, u.String(), harbor.Username, harbor.Password, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("[%s] post response close error %v", u.String(), err)
		}
	}()

	return io.ReadAll(resp.Body)
}

func get(ctx context.Context, url, username, password string) ([]byte, error) {
	req, err := makeHTTPRequest(ctx, http.MethodGet, url, username, password, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("[%s] get response close error %v", url, err)
		}
	}()

	return io.ReadAll(resp.Body)
}

func fetch(ctx context.Context, harbor HarborConfig, ref string) ([]byte, error) {
	u, err := urlpkg.Parse(harbor.URL)
	if err != nil {
		return nil, err
	}

	u, err = u.Parse(ref)
	if err != nil {
		return nil, err
	}

	return get(ctx, u.String(), harbor.Username, harbor.Password)
}

func fetchPage(ctx context.Context, harbor HarborConfig, ref string, callback func([]byte) error) error {
	u, err := urlpkg.Parse(harbor.URL)
	if err != nil {
		return err
	}

	u, err = u.Parse(ref)
	if err != nil {
		return err
	}

	for page := 1; ; page++ {
		query := u.Query()
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", "50")
		u.RawQuery = query.Encode()

		data, err := get(ctx, u.String(), harbor.Username, harbor.Password)
		if err != nil {
			return err
		}

		err = callback(data)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
	}
}

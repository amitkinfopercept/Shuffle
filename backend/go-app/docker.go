package main

// Docker
import (
	"github.com/frikky/shuffle-shared"

	"archive/tar"
	"path/filepath"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-git/go-billy/v5"

	network "github.com/docker/docker/api/types/network"
	natting "github.com/docker/go-connections/nat"

	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// Parses a directory with a Dockerfile into a tar for Docker images..
func getParsedTar(tw *tar.Writer, baseDir, extra string) error {
	return filepath.Walk(baseDir, func(file string, fi os.FileInfo, err error) error {
		if file == baseDir {
			return nil
		}

		//log.Printf("File: %s", file)
		//log.Printf("Fileinfo: %#v", fi)
		switch mode := fi.Mode(); {
		case mode.IsDir():
			// do directory recursion
			//log.Printf("DIR: %s", file)

			// Append "src" as extra here
			filenamesplit := strings.Split(file, "/")
			filename := fmt.Sprintf("%s%s/", extra, filenamesplit[len(filenamesplit)-1])

			tmpExtra := fmt.Sprintf(filename)
			//log.Printf("TmpExtra: %s", tmpExtra)
			err = getParsedTar(tw, file, tmpExtra)
			if err != nil {
				log.Printf("Directory parse issue: %s", err)
				return err
			}
		case mode.IsRegular():
			// do file stuff
			//log.Printf("FILE: %s", file)

			fileReader, err := os.Open(file)
			if err != nil {
				return err
			}

			// Read the actual Dockerfile
			readFile, err := ioutil.ReadAll(fileReader)
			if err != nil {
				log.Printf("Not file: %s", err)
				return err
			}

			filenamesplit := strings.Split(file, "/")
			filename := fmt.Sprintf("%s%s", extra, filenamesplit[len(filenamesplit)-1])
			//log.Printf("Filename: %s", filename)
			tarHeader := &tar.Header{
				Name: filename,
				Size: int64(len(readFile)),
			}

			//Writes the header described for the TAR file
			err = tw.WriteHeader(tarHeader)
			if err != nil {
				return err
			}

			// Writes the dockerfile data to the TAR file
			_, err = tw.Write(readFile)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Custom TAR builder in memory for Docker images
func getParsedTarMemory(fs billy.Filesystem, tw *tar.Writer, baseDir, extra string) error {
	// This one has to use baseDir + Extra
	newBase := fmt.Sprintf("%s%s", baseDir, extra)
	dir, err := fs.ReadDir(newBase)
	if err != nil {
		return err
	}

	for _, file := range dir {
		// Folder?
		switch mode := file.Mode(); {
		case mode.IsDir():
			filename := file.Name()
			filenamesplit := strings.Split(filename, "/")

			tmpExtra := fmt.Sprintf("%s%s/", extra, filenamesplit[len(filenamesplit)-1])
			//log.Printf("EXTRA: %s", tmpExtra)
			err = getParsedTarMemory(fs, tw, baseDir, tmpExtra)
			if err != nil {
				log.Printf("Directory parse issue: %s", err)
				return err
			}
		case mode.IsRegular():
			filenamesplit := strings.Split(file.Name(), "/")
			filename := fmt.Sprintf("%s%s", extra, filenamesplit[len(filenamesplit)-1])
			// Newbase
			path := fmt.Sprintf("%s%s", newBase, file.Name())

			fileReader, err := fs.Open(path)
			if err != nil {
				return err
			}

			//log.Printf("FILENAME: %s", filename)
			readFile, err := ioutil.ReadAll(fileReader)
			if err != nil {
				log.Printf("Not file: %s", err)
				return err
			}

			// Fixes issues with older versions of Docker and reference formats
			// Specific to Shuffle rn. Could expand.
			// FIXME: Seems like the issue was with multi-stage builds
			/*
				if filename == "Dockerfile" {
					log.Printf("Should search and replace in readfile.")

					referenceCheck := "FROM frikky/shuffle:"
					if strings.Contains(string(readFile), referenceCheck) {
						log.Printf("SHOULD SEARCH & REPLACE!")
						newReference := fmt.Sprintf("FROM registry.hub.docker.com/frikky/shuffle:")
						readFile = []byte(strings.Replace(string(readFile), referenceCheck, newReference, -1))
					}
				}
			*/

			//log.Printf("Filename: %s", filename)
			// FIXME - might need the folder from EXTRA here
			// Name has to be e.g. just "requirements.txt"
			tarHeader := &tar.Header{
				Name: filename,
				Size: int64(len(readFile)),
			}

			//Writes the header described for the TAR file
			err = tw.WriteHeader(tarHeader)
			if err != nil {
				return err
			}

			// Writes the dockerfile data to the TAR file
			_, err = tw.Write(readFile)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

/*
// Fixes App SDK issues.. meh
func fixTags(tags []string) []string {
	checkTag := "frikky/shuffle"
	newTags := []string{}
	for _, tag := range tags {
		if strings.HasPrefix(tag, checkTags) {
			newTags.append(newTags, fmt.Sprintf("registry.hub.docker.com/%s", tag))
		}

		newTags.append(tag)
	}
}
*/

// Custom Docker image builder wrapper in memory
func buildImageMemory(fs billy.Filesystem, tags []string, dockerfileFolder string, downloadIfFail bool) error {
	ctx := context.Background()
	client, err := client.NewEnvClient()
	if err != nil {
		log.Printf("Unable to create docker client: %s", err)
		return err
	}

	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	log.Printf("[INFO] Setting up memory build structure for folder: %s", dockerfileFolder)
	err = getParsedTarMemory(fs, tw, dockerfileFolder, "")
	if err != nil {
		log.Printf("Tar issue: %s", err)
		return err
	}

	dockerFileTarReader := bytes.NewReader(buf.Bytes())

	// Dockerfile is inside the TAR itself. Not local context
	// docker build --build-arg http_proxy=http://my.proxy.url
	buildOptions := types.ImageBuildOptions{
		Remove:    true,
		Tags:      tags,
		BuildArgs: map[string]*string{},
	}
	// NetworkMode: "host",

	httpProxy := os.Getenv("HTTP_PROXY")
	if len(httpProxy) > 0 {
		buildOptions.BuildArgs["http_proxy"] = &httpProxy
	}
	httpsProxy := os.Getenv("HTTPS_PROXY")
	if len(httpProxy) > 0 {
		buildOptions.BuildArgs["https_proxy"] = &httpsProxy
	}

	// Build the actual image
	log.Printf("[INFO] Building %s. This may take up to a few minutes.", dockerfileFolder)
	imageBuildResponse, err := client.ImageBuild(
		ctx,
		dockerFileTarReader,
		buildOptions,
	)

	//log.Printf("Response: %#v", imageBuildResponse.Body)
	//log.Printf("IMAGERESPONSE: %#v", imageBuildResponse.Body)

	defer imageBuildResponse.Body.Close()
	buildBuf := new(strings.Builder)
	_, newerr := io.Copy(buildBuf, imageBuildResponse.Body)
	if newerr != nil {
		log.Printf("Failed reading Docker build STDOUT: %s", newerr)
	} else {
		log.Printf("STRING: %s", buildBuf.String())
		if strings.Contains(buildBuf.String(), "errorDetail") {
			log.Printf("[ERROR] Docker build:\n%s\nERROR ABOVE: Trying to pull tags from: %s", buildBuf.String(), strings.Join(tags, "\n"))

			// Handles pulling of the same image if applicable
			// This fixes some issues with older versions of Docker which can't build
			// on their own ( <17.05 )
			pullOptions := types.ImagePullOptions{}
			downloaded := false
			for _, image := range tags {
				// Is this ok? Not sure. Tags shouldn't be controlled here prolly.
				image = strings.ToLower(image)

				newImage := fmt.Sprintf("%s/%s", registryName, image)
				log.Printf("[INFO] Pulling image %s", newImage)
				reader, err := client.ImagePull(ctx, newImage, pullOptions)
				if err != nil {
					log.Printf("[ERROR] Failed getting image %s: %s", newImage, err)
					continue
				}

				// Attempt to retag the image to not contain registry...

				//newBuf := buildBuf
				downloaded = true
				io.Copy(os.Stdout, reader)
				log.Printf("[INFO] Successfully downloaded and built %s", newImage)
			}

			if !downloaded {
				return errors.New(fmt.Sprintf("Failed to build / download images %s", strings.Join(tags, ",")))
			}
			//baseDockerName
		}
	}

	if err != nil {
		// Read the STDOUT from the build process

		return err
	}

	return nil
}

func buildImage(tags []string, dockerfileFolder string) error {
	ctx := context.Background()
	client, err := client.NewEnvClient()
	if err != nil {
		log.Printf("Unable to create docker client: %s", err)
		return err
	}

	log.Printf("[INFO] Docker Tags: %s", tags)
	dockerfileSplit := strings.Split(dockerfileFolder, "/")

	// Create a buffer
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()
	baseDir := strings.Join(dockerfileSplit[0:len(dockerfileSplit)-1], "/")

	// Builds the entire folder into buf
	err = getParsedTar(tw, baseDir, "")
	if err != nil {
		log.Printf("Tar issue: %s", err)
	}

	dockerFileTarReader := bytes.NewReader(buf.Bytes())
	buildOptions := types.ImageBuildOptions{
		Remove:    true,
		Tags:      tags,
		BuildArgs: map[string]*string{},
	}
	//NetworkMode: "host",

	httpProxy := os.Getenv("HTTP_PROXY")
	if len(httpProxy) > 0 {
		buildOptions.BuildArgs["http_proxy"] = &httpProxy
	}
	httpsProxy := os.Getenv("HTTPS_PROXY")
	if len(httpProxy) > 0 {
		buildOptions.BuildArgs["https_proxy"] = &httpsProxy
	}

	// Build the actual image
	imageBuildResponse, err := client.ImageBuild(
		ctx,
		dockerFileTarReader,
		buildOptions,
	)

	if err != nil {
		return err
	}

	// Read the STDOUT from the build process
	defer imageBuildResponse.Body.Close()
	buildBuf := new(strings.Builder)
	_, err = io.Copy(buildBuf, imageBuildResponse.Body)
	if err != nil {
		return err
	} else {
		if strings.Contains(buildBuf.String(), "errorDetail") {
			log.Printf("[ERROR] Docker build:\n%s\nERROR ABOVE: Trying to pull tags from: %s", buildBuf.String(), strings.Join(tags, "\n"))
			return errors.New(fmt.Sprintf("Failed building %s. Check backend logs for details. Most likely means you have an old version of Docker.", strings.Join(tags, ",")))
		}
	}

	return nil
}

// FIXME - very specific for webhooks. Make it easier?
func stopWebhook(image string, identifier string) error {
	ctx := context.Background()

	containername := fmt.Sprintf("%s-%s", image, identifier)

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Println("Unable to create docker client")
		return err
	}

	//	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
	//		All: true,
	//	})

	if err := cli.ContainerStop(ctx, containername, nil); err != nil {
		log.Printf("Unable to stop container %s - running removal anyway, just in case: %s", containername, err)
	}

	removeOptions := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	if err := cli.ContainerRemove(ctx, containername, removeOptions); err != nil {
		log.Printf("Unable to remove container: %s", err)
	}

	return nil
}

// FIXME - remember to set DOCKER_API_VERSION
// FIXME - remove github.com/docker/docker/vendor
// FIXME - Library dependencies for NAT is fucked..
// https://docs.docker.com/develop/sdk/examples/
func deployWebhook(image string, identifier string, path string, port string, callbackurl string, apikey string) error {
	cli, err := client.NewEnvClient()
	if err != nil {
		fmt.Println("Unable to create docker client")
		return err
	}

	newport, err := natting.NewPort("tcp", port)
	if err != nil {
		fmt.Println("Unable to create docker port")
		return err
	}

	// FIXME - logging?

	hostConfig := &container.HostConfig{
		PortBindings: natting.PortMap{
			newport: []natting.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: port,
				},
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
		LogConfig: container.LogConfig{
			Type:   "json-file",
			Config: map[string]string{},
		},
	}

	//networkConfig := &network.NetworkSettings{}
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{},
	}

	test := &network.EndpointSettings{
		Gateway: "helo",
	}

	networkConfig.EndpointsConfig["bridge"] = test

	exposedPorts := map[natting.Port]struct{}{
		newport: struct{}{},
	}

	config := &container.Config{
		Image: image,
		Env: []string{
			fmt.Sprintf("URIPATH=%s", path),
			fmt.Sprintf("HOOKPORT=%s", port),
			fmt.Sprintf("CALLBACKURL=%s", callbackurl),
			fmt.Sprintf("APIKEY=%s", apikey),
			fmt.Sprintf("HOOKID=%s", identifier),
		},
		ExposedPorts: exposedPorts,
		Hostname:     fmt.Sprintf("%s-%s", image, identifier),
	}

	cont, err := cli.ContainerCreate(
		context.Background(),
		config,
		hostConfig,
		networkConfig,
		fmt.Sprintf("%s-%s", image, identifier),
	)

	if err != nil {
		log.Println(err)
		return err
	}

	cli.ContainerStart(context.Background(), cont.ID, types.ContainerStartOptions{})
	log.Printf("Container %s is created", cont.ID)
	return nil
}

// Starts a new webhook
func handleStopHookDocker(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var fileId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		fileId = location[4]
	}

	if len(fileId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	ctx := context.Background()
	hook, err := getHook(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting hook %s (stop docker): %s", fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	log.Printf("Status: %s", hook.Status)
	log.Printf("Running: %t", hook.Running)
	if !hook.Running {
		message := fmt.Sprintf("Error: %s isn't running", hook.Id)
		log.Println(message)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "message": "%s"}`, message)))
		return
	}

	hook.Status = "stopped"
	hook.Running = false
	hook.Actions = []HookAction{}
	err = setHook(ctx, *hook)
	if err != nil {
		log.Printf("Failed setting hook: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	image := "webhook"

	// This is here to force stop and remove the old webhook
	err = stopWebhook(image, fileId)
	if err != nil {
		log.Printf("Container stop issue for %s-%s: %s", image, fileId, err)
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true, "message": "Stopped webhook"}`))
}

// THis is an example
// Can also be used as base data?
var webhook = `{
	"id": "d6ef8912e8bd37776e654cbc14c2629c",
	"info": {
		"url": "http://localhost:5001",
		"name": "TheHive",
		"description": "Webhook for TheHive"
	},
	"transforms": {},
	"actions": {},
	"type": "webhook",
	"running": false,
	"status": "stopped"
}`

// Starts a new webhook
func handleDeleteHookDocker(resp http.ResponseWriter, request *http.Request) {
	ctx := context.Background()
	cors := handleCors(resp, request)
	if cors {
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var fileId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		fileId = location[4]
	}

	if len(fileId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	err := DeleteKey(ctx, "hooks", fileId)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "Can't delete"}`))
		return
	}

	image := "webhook"

	// This is here to force stop and remove the old webhook
	err = stopWebhook(image, fileId)
	if err != nil {
		log.Printf("Container stop issue for %s-%s: %s", image, fileId, err)
		resp.Write([]byte(`{"success": false, "message": "Couldn't stop webhook"}`))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true, "message": "Deleted webhook"}`))
}

// Starts a new webhook
func handleStartHookDocker(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var fileId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		fileId = location[4]
	}

	if len(fileId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	ctx := context.Background()
	hook, err := getHook(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting hook %s (start docker): %s", fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if len(hook.Info.Url) == 0 {
		log.Printf("Hook url can't be empty.")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	log.Printf("Status: %s", hook.Status)
	log.Printf("Running: %t", hook.Running)
	if hook.Running || hook.Status == "Running" {
		message := fmt.Sprintf("Error: %s is already running", hook.Id)
		log.Println(message)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "message": "%s"}`, message)))
		return
	}

	// FIXME - verify?
	// FIXME - static port? Generate from available range.
	image := "webhook"
	filepath := "/webhook"
	baseUrl := "http://localhost"
	callbackUrl := "http://localhost:8001"

	// This is here to force stop and remove the old webhook
	err = stopWebhook(image, fileId)
	if err != nil {
		log.Printf("Container stop issue for %s-%s: %s", image, fileId, err)
	}

	// Dynamic ish ports
	var startPort int64 = 5001
	var endPort int64 = 5010
	port := findAvailablePorts(startPort, endPort)
	if len(port) == 0 {
		message := fmt.Sprintf("Not ports available in the range %d-%d", startPort, endPort)
		log.Println(message)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "message": "%s"}`, message)))
		return

	}

	hook.Status = "running"
	hook.Running = true

	// Set this for more than just hooks?
	if hook.Type == "webhook" {
		hook.Info.Url = fmt.Sprintf("%s:%s%s", baseUrl, port, filepath)
	}
	err = setHook(ctx, *hook)
	if err != nil {
		log.Printf("Failed setting hook: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Cloud run? Let's make a generic webhook that can be deployed easily
	log.Printf("Should run a webhook with the following: \nUrl: %s\nId: %s\n", hook.Info.Url, hook.Id)

	// FIXME - set port based on what the user specified / what was generated
	// FIXME - add nonstatic APIKEY
	apiKey := "ASD"

	err = deployWebhook(image, fileId, filepath, port, callbackUrl, apiKey)
	if err != nil {
		log.Printf("Failed starting container %s-%s: %s", image, fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - get some real data?
	log.Printf("[INFO] Successfully started %s-%s on port %s with filepath %s", image, fileId, port, filepath)
	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true, "message": "Started webhook"}`))
	return
}

// Checks if an image exists
func imageCheckBuilder(images []string) error {
	//log.Printf("[FIXME] ImageNames to check: %#v", images)
	return nil

	ctx := context.Background()
	client, err := client.NewEnvClient()
	if err != nil {
		log.Printf("Unable to create docker client: %s", err)
		return err
	}

	allImages, err := client.ImageList(ctx, types.ImageListOptions{
		All: true,
	})

	if err != nil {
		log.Printf("[ERROR] Failed creating imagelist: %s", err)
		return err
	}

	filteredImages := []types.ImageSummary{}
	for _, image := range allImages {
		found := false
		for _, repoTag := range image.RepoTags {
			if strings.Contains(repoTag, baseDockerName) {
				found = true
				break
			}
		}

		if found {
			filteredImages = append(filteredImages, image)
		}
	}

	// FIXME: Continue fixing apps here
	// https://github.com/frikky/Shuffle/issues/135
	// 1. Find if app exists
	// 2. Create app if it doesn't
	//log.Printf("Apps: %#v", filteredImages)

	return nil
}

func hookTest() {
	var hook Hook
	err := json.Unmarshal([]byte(webhook), &hook)
	log.Println(webhook)
	if err != nil {
		log.Printf("Failed hook unmarshaling: %s", err)
		return
	}

	ctx := context.Background()
	err = setHook(ctx, hook)
	if err != nil {
		log.Printf("Failed setting hook: %s", err)
	}

	returnHook, err := getHook(ctx, hook.Id)
	if err != nil {
		log.Printf("Failed getting hook %s (test): %s", hook.Id, err)
	}

	if len(returnHook.Id) > 0 {
		log.Printf("Success! - %s", returnHook.Id)
	}
}

//https://stackoverflow.com/questions/23935141/how-to-copy-docker-images-from-one-host-to-another-without-using-a-repository
func getDockerImage(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// Just here to verify that the user is logged in
	_, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in validate swagger: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Failed reading body"}`))
		return
	}

	type requestCheck struct {
		Name string `datastore:"name" json:"name" yaml:"name"`
	}

	//body = []byte(`swagger: "2.0"`)
	//body = []byte(`swagger: '1.0'`)
	//newbody := string(body)
	//newbody = strings.TrimSpace(newbody)
	//body = []byte(newbody)
	//log.Println(string(body))
	//tmpbody, err := yaml.YAMLToJSON(body)
	//log.Println(err)
	//log.Println(string(tmpbody))

	// This has to be done in a weird way because Datastore doesn't
	// support map[string]interface and similar (openapi3.Swagger)
	var version requestCheck

	err = json.Unmarshal(body, &version)
	if err != nil {
		resp.WriteHeader(422)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed JSON marshalling: %s"}`, err)))
		return
	}

	log.Printf("Image to load: %s", version.Name)
	//cli, err := client.NewEnvClient()
	//if err != nil {
	//	log.Println("Unable to create docker client")
	//	return err
	//}

	dockercli, err := client.NewEnvClient()
	if err != nil {
		log.Printf("Unable to create docker client: %s", err)
		resp.WriteHeader(422)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed JSON marshalling: %s"}`, err)))
		return
	}

	ctx := context.Background()
	images, err := dockercli.ImageList(ctx, types.ImageListOptions{
		All: true,
	})

	img := types.ImageSummary{}
	tagFound := ""
	for _, image := range images {
		for _, tag := range image.RepoTags {
			log.Printf("[INFO] Docker Image: %s", tag)

			if strings.ToLower(tag) == strings.ToLower(version.Name) {
				img = image
				tagFound = tag
				break
			}
		}
	}

	if len(img.ID) == 0 {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "message": "Couldn't find image %s"}`, version.Name)))
		return
	}
	_ = tagFound

	/*
		log.Printf("IMg: %#v", img)
		pullOptions := types.ImagePullOptions{}
		log.Printf("[INFO] Pulling image %s", image)
		reader, err := dockercli.ImagePull(ctx, tag, pullOptions)
		if err != nil {
			log.Printf("[ERROR] Failed getting image %s: %s", image, err)
		}

		io.Copy(os.Stdout, r)
	*/

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true, "message": "Downloading image %s"}`, version.Name)))
}

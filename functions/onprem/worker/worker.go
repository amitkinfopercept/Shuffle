package main

import (
	"github.com/frikky/shuffle-shared"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	//"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	dockerclient "github.com/docker/docker/client"
	"github.com/satori/go.uuid"

	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
)

// This is getting out of hand :)
var environment = os.Getenv("ENVIRONMENT_NAME")
var baseUrl = os.Getenv("BASE_URL")
var appCallbackUrl = os.Getenv("BASE_URL")
var cleanupEnv = strings.ToLower(os.Getenv("CLEANUP"))
var baseimagename = "frikky/shuffle"
var registryName = "registry.hub.docker.com"
var fallbackName = "shuffle-orborus"
var sleepTime = 2
var requestCache *cache.Cache
var topClient *http.Client
var data string
var requestsSent = 0

var environments []string
var parents map[string][]string
var children map[string][]string
var visited []string
var executed []string
var nextActions []string
var containerIds []string
var extra int
var startAction string

var containerId string

// form container id of current running container
func getThisContainerId() string {
	if len(containerId) > 0 {
		return containerId
	}

	id := ""
	cmd := fmt.Sprintf("cat /proc/self/cgroup | grep memory | tail -1 | cut -d/ -f3 | grep -o -E '[0-9A-z]{64}'")
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err == nil {
		id = strings.TrimSpace(string(out))

		//log.Printf("Checking if %s is in %s", ".scope", string(out))
		if strings.Contains(string(out), ".scope") {
			id = fallbackName
		}
	}

	return id
}

func init() {
	containerId = getThisContainerId()
	if len(containerId) == 0 {
		log.Printf("[WARNING] No container ID found. Not running containerized? This should only show during testing")
	} else {
		log.Printf("[INFO] Found container ID for this worker: %s", containerId)
	}
}

// removes every container except itself (worker)
func shutdown(workflowExecution shuffle.WorkflowExecution, nodeId string, reason string, handleResultSend bool) {
	log.Printf("[INFO] Shutdown (%s) started with reason %s", workflowExecution.Status, reason)
	//reason := "Error in execution"

	sleepDuration := 1
	if handleResultSend && requestsSent < 2 {
		data, err := json.Marshal(workflowExecution)
		if err == nil {
			sendResult(workflowExecution, data)
			log.Printf("[WARNING] Sent shutdown update")
		} else {
			log.Printf("[WARNING] DIDNT send update")
		}

		time.Sleep(time.Duration(sleepDuration) * time.Second)
	}

	// Might not be necessary because of cleanupEnv hostconfig autoremoval
	if cleanupEnv == "true" && len(containerIds) > 0 {
		/*
			ctx := context.Background()
			dockercli, err := dockerclient.NewEnvClient()
			if err == nil {
				log.Printf("[INFO] Cleaning up %d containers", len(containerIds))
				removeOptions := types.ContainerRemoveOptions{
					RemoveVolumes: true,
					Force:         true,
				}

				for _, containername := range containerIds {
					log.Printf("[INFO] Should stop and and remove container %s (deprecated)", containername)
					//dockercli.ContainerStop(ctx, containername, nil)
					//dockercli.ContainerRemove(ctx, containername, removeOptions)
					//removeContainers = append(removeContainers, containername)
				}
			}
		*/
	} else {
		log.Printf("[INFO] NOT cleaning up containers. IDS: %d, CLEANUP env: %s", len(containerIds), cleanupEnv)
	}

	fullUrl := fmt.Sprintf("%s/api/v1/workflows/%s/executions/%s/abort", baseUrl, workflowExecution.Workflow.ID, workflowExecution.ExecutionId)

	path := fmt.Sprintf("?reason=%s", url.QueryEscape(reason))
	if len(nodeId) > 0 {
		path += fmt.Sprintf("&node=%s", url.QueryEscape(nodeId))
	}
	if len(environment) > 0 {
		path += fmt.Sprintf("&env=%s", url.QueryEscape(environment))
	}

	//fmt.Println(url.QueryEscape(query))
	fullUrl += path
	log.Printf("[INFO] Abort URL: %s", fullUrl)

	req, err := http.NewRequest(
		"GET",
		fullUrl,
		nil,
	)

	if err != nil {
		log.Println("[INFO] Failed building request: %s", err)
	}

	// FIXME: Add an API call to the backend
	authorization := os.Getenv("AUTHORIZATION")
	if len(authorization) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", authorization))
	} else {
		log.Printf("[ERROR] No authorization specified for abort")
	}

	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
		},
	}

	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxy := os.Getenv("HTTPS_PROXY")
	if (len(httpProxy) > 0 || len(httpsProxy) > 0) && baseUrl != "http://shuffle-backend:5001" {
		client = &http.Client{}
	} else {
		if len(httpProxy) > 0 {
			log.Printf("[INFO] Running with HTTP proxy %s (env: HTTP_PROXY)", httpProxy)
		}
		if len(httpsProxy) > 0 {
			log.Printf("[INFO] Running with HTTPS proxy %s (env: HTTPS_PROXY)", httpsProxy)
		}
	}
	_, err = client.Do(req)
	if err != nil {
		log.Printf("[INFO] Failed abort request: %s", err)
	}

	log.Printf("[INFO] Finished shutdown (after %d seconds).", sleepDuration)
	// Allows everything to finish in subprocesses
	time.Sleep(time.Duration(sleepDuration) * time.Second)
	os.Exit(3)
}

// Deploys the internal worker whenever something happens
func deployApp(cli *dockerclient.Client, image string, identifier string, env []string, workflowExecution shuffle.WorkflowExecution) error {
	// form basic hostConfig
	ctx := context.Background()
	hostConfig := &container.HostConfig{
		LogConfig: container.LogConfig{
			Type:   "json-file",
			Config: map[string]string{},
		},
		Resources: container.Resources{
			CPUShares: 256,
			CPUPeriod: 10000,
		},
	}

	// form container id and use it as network source if it's not empty
	containerId = getThisContainerId()
	if containerId != "" {
		hostConfig.NetworkMode = container.NetworkMode(fmt.Sprintf("container:%s", containerId))
	} else {
		log.Printf("[WARNING] Empty self container id, continue without NetworkMode")
	}

	// Removing because log extraction should happen first
	if cleanupEnv == "true" {
		hostConfig.AutoRemove = true
	}

	// FIXME: Add proper foldermounts here
	//log.Printf("\n\nPRE FOLDERMOUNT\n\n")
	//volumeBinds := []string{"/tmp/shuffle-mount:/rules"}
	//volumeBinds := []string{"/tmp/shuffle-mount:/rules"}
	volumeBinds := []string{}
	if len(volumeBinds) > 0 {
		log.Printf("[INFO] Setting up binds for container!")
		hostConfig.Binds = volumeBinds
		hostConfig.Mounts = []mount.Mount{}
		for _, bind := range volumeBinds {
			if !strings.Contains(bind, ":") || strings.Contains(bind, "..") || strings.HasPrefix(bind, "~") {
				log.Printf("[WARNING] Bind %s is invalid.", bind)
				continue
			}

			log.Printf("[INFO] Appending bind %s", bind)
			bindSplit := strings.Split(bind, ":")
			sourceFolder := bindSplit[0]
			destinationFolder := bindSplit[0]
			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: sourceFolder,
				Target: destinationFolder,
			})
		}
	} else {
		log.Printf("[WARNING] No mounted folders")
	}
	//	hostConfig.Binds = volumeBinds
	//}

	config := &container.Config{
		Image: image,
		Env:   env,
	}

	cont, err := cli.ContainerCreate(
		ctx,
		config,
		hostConfig,
		nil,
		nil,
		identifier,
	)

	if err != nil {
		log.Printf("[WARNING] Container CREATE error: %s", err)
		return err
	}

	err = cli.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{})
	if err != nil {
		log.Printf("[ERROR] Failed to start container in environment %s: %s", environment, err)
		//shutdown(workflowExecution, workflowExecution.Workflow.ID, true)
		return err
	}

	log.Printf("[INFO] Container %s was created for %s", cont.ID, identifier)

	// Waiting to see if it exits.. Stupid, but stable(r)
	if workflowExecution.ExecutionSource != "default" {
		log.Printf("[INFO] Handling NON-default execution source %s - NOT waiting and validating!", workflowExecution.ExecutionSource)
	} else if workflowExecution.ExecutionSource == "default" {
		time.Sleep(2 * time.Second)

		stats, err := cli.ContainerInspect(ctx, cont.ID)
		if err != nil {
			log.Printf("[ERROR] Failed getting container stats")
		} else {
			//log.Printf("[INFO] Info for container: %#v", stats)
			//log.Printf("%#v", stats.Config)
			//log.Printf("%#v", stats.ContainerJSONBase.State)
			log.Printf("[INFO] EXECUTION STATUS: %s", stats.ContainerJSONBase.State.Status)
			if stats.ContainerJSONBase.State.Status == "exited" {
				logOptions := types.ContainerLogsOptions{
					ShowStdout: true,
				}

				out, err := cli.ContainerLogs(ctx, cont.ID, logOptions)
				if err != nil {
					log.Printf("[INFO] Failed getting logs: %s", err)
				} else {
					log.Printf("IN ELSE FOR DEPLOY")
					buf := new(strings.Builder)
					io.Copy(buf, out)
					logs := buf.String()
					log.Printf("Logs: %s", logs)

					//log.Printf(logs)
					// check errors
					/*
						if strings.Contains(logs, "Error") {
							log.Printf("ERROR IN %s?", cont.ID)
							log.Println(logs)
							//return errors.New(fmt.Sprintf("ERROR FROM CONTAINER %s", cont.ID))
						} else {
							log.Printf("NORMAL EXEC OF %s?", cont.ID)
						}
					*/
				}

				log.Printf("ERROR IN CONTAINER DEPLOYMENT - ITS EXITED!")

				return errors.New(fmt.Sprintf(`{"success": false, "reason": "Container %s exited prematurely.","debug": "docker logs -f %s"}`, cont.ID, cont.ID))
			}
		}
	}

	/*
		//log.Printf("%#v", stats.Config.Status)
		//ContainerJSONtoConfig(cj dockType.ContainerJSON) ContainerConfig {
			listOptions := types.ContainerListOptions{
				Filters: filters.Args{
					map[string][]string{"ancestor": {"<imagename>:<version>"}},
				},
			}
			containers, err := cli.ContainerList(ctx, listOptions)
	*/

	//log.Printf("%#v", cont.Status)
	//config := ContainerJSONtoConfig(stats)
	//log.Printf("CONFIG: %#v", config)

	/*
		logOptions := types.ContainerLogsOptions{
			ShowStdout: true,
		}

	*/

	containerIds = append(containerIds, cont.ID)
	return nil
}

func removeContainer(containername string) error {
	ctx := context.Background()

	cli, err := dockerclient.NewEnvClient()
	if err != nil {
		log.Printf("[INFO] Unable to create docker client: %s", err)
		return err
	}

	// FIXME - ucnomment
	//	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
	//		All: true,
	//	})

	_ = ctx
	_ = cli
	//if err := cli.ContainerStop(ctx, containername, nil); err != nil {
	//	log.Printf("Unable to stop container %s - running removal anyway, just in case: %s", containername, err)
	//}

	removeOptions := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	// FIXME - remove comments etc
	_ = removeOptions
	//if err := cli.ContainerRemove(ctx, containername, removeOptions); err != nil {
	//	log.Printf("Unable to remove container: %s", err)
	//}

	return nil
}

func runFilter(workflowExecution shuffle.WorkflowExecution, action shuffle.Action) {
	// 1. Get the parameter $.#.id
	if action.Label == "filter_cases" && len(action.Parameters) > 0 {
		if action.Parameters[0].Variant == "ACTION_RESULT" {
			param := action.Parameters[0]
			value := param.Value
			_ = value

			// Loop cases.. Hmm, that's tricky
		}
	} else {
		log.Printf("No handler for filter %s with %d params", action.Label, len(action.Parameters))
	}

}

func handleSubworkflowExecution(client *http.Client, workflowExecution shuffle.WorkflowExecution, action shuffle.Trigger, baseAction shuffle.Action) error {
	apikey := ""
	workflowId := ""
	executionArgument := ""
	for _, parameter := range action.Parameters {
		log.Printf("Parameter name: %s", parameter.Name)
		if parameter.Name == "user_apikey" {
			apikey = parameter.Value
		} else if parameter.Name == "workflow" {
			workflowId = parameter.Value
		} else if parameter.Name == "data" {
			executionArgument = parameter.Value
		}
	}

	//handleSubworkflowExecution(workflowExecution, action)
	status := "SUCCESS"
	baseResult := `{"success": true}`
	if len(apikey) == 0 || len(workflowId) == 0 {
		status = "FAILURE"
		baseResult = `{"success": false}`
	} else {
		log.Printf("Should execute workflow %s with APIKEY %s and data %s", workflowId, apikey, executionArgument)
		fullUrl := fmt.Sprintf("%s/api/workflows/%s/execute", baseUrl, workflowId)
		req, err := http.NewRequest(
			"POST",
			fullUrl,
			bytes.NewBuffer([]byte(executionArgument)),
		)

		if err != nil {
			log.Printf("Error building test request: %s", err)
			return err
		}

		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apikey))
		newresp, err := client.Do(req)
		if err != nil {
			log.Printf("Error running test request: %s", err)
			return err
		}

		body, err := ioutil.ReadAll(newresp.Body)
		if err != nil {
			log.Printf("Failed reading body when waiting: %s", err)
			return err
		}

		log.Printf("Execution Result: %s", body)
	}

	timeNow := time.Now().Unix()
	//curaction := shuffle.Action{
	//	AppName:    baseAction.AppName,
	//	AppVersion: baseAction.AppVersion,
	//	Label:      baseAction.Label,
	//	Name:       baseAction.Name,
	//	ID:         baseAction.ID,
	//}
	result := shuffle.ActionResult{
		Action:        baseAction,
		ExecutionId:   workflowExecution.ExecutionId,
		Authorization: workflowExecution.Authorization,
		Result:        baseResult,
		StartedAt:     timeNow,
		CompletedAt:   0,
		Status:        status,
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return err
	}

	fullUrl := fmt.Sprintf("%s/api/v1/streams", baseUrl)
	req, err := http.NewRequest(
		"POST",
		fullUrl,
		bytes.NewBuffer([]byte(resultData)),
	)

	if err != nil {
		log.Printf("Error building test request: %s", err)
		return err
	}

	newresp, err := client.Do(req)
	if err != nil {
		log.Printf("Error running test request: %s", err)
		return err
	}

	body, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		log.Printf("Failed reading body when waiting: %s", err)
		return err
	}

	log.Printf("[INFO] Subworkflow Body: %s", string(body))

	if status == "FAILURE" {
		return errors.New("[ERROR] Failed to execute subworkflow")
	} else {
		return nil
	}
}

func removeIndex(s []string, i int) []string {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

func handleExecutionResult(workflowExecution shuffle.WorkflowExecution) {
	if len(startAction) == 0 {
		startAction = workflowExecution.Start
		if len(startAction) == 0 {
			log.Printf("Didn't find execution start action. Setting it to workflow start action.")
			startAction = workflowExecution.Workflow.Start
		}
	}

	//log.Printf("NEXTACTIONS: %s", nextActions)
	queueNodes := []string{}
	//if len(nextActions) == 0 {
	//	nextActions = append(nextActions, startAction)
	//}

	if len(workflowExecution.Results) == 0 {
		nextActions = []string{startAction}
	} else {
		// This is to re-check the nodes that exist and whether they should continue
		appendActions := []string{}
		for _, item := range workflowExecution.Results {

			// FIXME: Check whether the item should be visited or not
			// Do the same check as in walkoff.go - are the parents done?
			// If skipped and both parents are skipped: keep as skipped, otherwise queue
			if item.Status == "SKIPPED" {
				isSkipped := true

				for _, branch := range workflowExecution.Workflow.Branches {
					// 1. Finds branches where the destination is our node
					// 2. Finds results of those branches, and sees the status
					// 3. If the status isn't skipped or failure, then it will still run this node
					if branch.DestinationID == item.Action.ID {
						for _, subresult := range workflowExecution.Results {
							if subresult.Action.ID == branch.SourceID {
								if subresult.Status != "SKIPPED" && subresult.Status != "FAILURE" {
									log.Printf("\n\n\nSUBRESULT PARENT STATUS: %s\n\n\n", subresult.Status)
									isSkipped = false

									break
								}
							}
						}
					}
				}

				if isSkipped {
					//log.Printf("Skipping %s as all parents are done", item.Action.Label)
					if !arrayContains(visited, item.Action.ID) {
						log.Printf("[INFO] Adding visited (1): %s", item.Action.Label)
						visited = append(visited, item.Action.ID)
					}
				} else {
					log.Printf("[INFO] Continuing %s as all parents are NOT done", item.Action.Label)
					appendActions = append(appendActions, item.Action.ID)
				}
			} else {
				if item.Status == "FINISHED" {
					log.Printf("[INFO] Adding visited (2): %s", item.Action.Label)
					visited = append(visited, item.Action.ID)
				}
			}

			//if len(nextActions) == 0 {
			//nextActions = append(nextActions, children[item.Action.ID]...)
			for _, child := range children[item.Action.ID] {
				if !arrayContains(nextActions, child) && !arrayContains(visited, child) && !arrayContains(visited, child) {
					nextActions = append(nextActions, child)
				}
			}

			if len(appendActions) > 0 {
				log.Printf("APPENDED NODES: %#v", appendActions)
				nextActions = append(nextActions, appendActions...)
			}
		}
	}

	//log.Printf("Nextactions: %s", nextActions)
	// This is a backup in case something goes wrong in this complex hellhole.
	// Max default execution time is 5 minutes for now anyway, which should take
	// care if it gets stuck in a loop.
	// FIXME: Force killing a worker should result in a notification somewhere
	if len(nextActions) == 0 {
		log.Printf("[INFO] No next action. Finished? Result vs shuffle.Actions: %d - %d", len(workflowExecution.Results), len(workflowExecution.Workflow.Actions))
		exit := true
		for _, item := range workflowExecution.Results {
			if item.Status == "EXECUTING" {
				exit = false
				break
			}
		}

		if len(environments) == 1 {
			log.Printf("[INFO] Should send results to the backend because environments are %s", environments)
			validateFinished(workflowExecution)
		}

		if exit && len(workflowExecution.Results) == len(workflowExecution.Workflow.Actions) {
			log.Printf("Shutting down.")
			shutdown(workflowExecution, "", "", true)
		}

		// Look for the NEXT missing action
		notFound := []string{}
		for _, action := range workflowExecution.Workflow.Actions {
			found := false
			for _, result := range workflowExecution.Results {
				if action.ID == result.Action.ID {
					found = true
					break
				}
			}

			if !found {
				notFound = append(notFound, action.ID)
			}
		}

		//log.Printf("SOMETHING IS MISSING!: %#v", notFound)
		for _, item := range notFound {
			if arrayContains(executed, item) {
				log.Printf("%s has already executed but no result!", item)
				return
			}

			// Visited means it's been touched in any way.
			outerIndex := -1
			for index, visit := range visited {
				if visit == item {
					outerIndex = index
					break
				}
			}

			if outerIndex >= 0 {
				log.Printf("Removing index %s from visited")
				visited = append(visited[:outerIndex], visited[outerIndex+1:]...)
			}

			fixed := 0
			for _, parent := range parents[item] {
				parentResult := getResult(workflowExecution, parent)
				if parentResult.Status == "FINISHED" || parentResult.Status == "SUCCESS" || parentResult.Status == "SKIPPED" || parentResult.Status == "FAILURE" {
					fixed += 1
				}
			}

			if fixed == len(parents[item]) {
				nextActions = append(nextActions, item)
			}

			// If it's not executed and not in nextActions
			// FIXME: Check if the item's parents are finished. If they're not, skip.
		}
	}

	//log.Printf("Checking nextactions: %s", nextActions)
	for _, node := range nextActions {
		nodeChildren := children[node]
		for _, child := range nodeChildren {
			if !arrayContains(queueNodes, child) {
				queueNodes = append(queueNodes, child)
			}
		}
	}

	// IF NOT VISITED && IN toExecuteOnPrem
	// SKIP if it's not onprem
	toRemove := []int{}
	//log.Printf("\n\nNEXTACTIONS: %#v\n\n", nextActions)
	for index, nextAction := range nextActions {
		action := getAction(workflowExecution, nextAction, environment)
		// check visited and onprem
		if arrayContains(visited, nextAction) {
			//log.Printf("ALREADY VISITIED (%s): %s", action.Label, nextAction)
			toRemove = append(toRemove, index)
			//nextActions = removeIndex(nextActions, index)

			//validateFinished(workflowExecution)
			_ = index

			continue
		}

		if action.AppName == "Shuffle Workflow" {
			//log.Printf("SHUFFLE WORKFLOW: %#v", action)
			action.Environment = environment
			action.AppName = "shuffle-subflow"
			action.Name = "run_subflow"
			action.AppVersion = "1.0.0"

			//appname := action.AppName
			//appversion := action.AppVersion
			//appname = strings.Replace(appname, ".", "-", -1)
			//appversion = strings.Replace(appversion, ".", "-", -1)
			//	shuffle-subflow_1.0.0

			//visited = append(visited, action.ID)
			//executed = append(executed, action.ID)

			trigger := shuffle.Trigger{}
			for _, innertrigger := range workflowExecution.Workflow.Triggers {
				if innertrigger.ID == action.ID {
					trigger = innertrigger
					break
				}
			}

			// FIXME: Add startnode from frontend
			action.Parameters = []shuffle.WorkflowAppActionParameter{}
			for _, parameter := range trigger.Parameters {
				parameter.Variant = "STATIC_VALUE"
				action.Parameters = append(action.Parameters, parameter)
			}

			action.Parameters = append(action.Parameters, shuffle.WorkflowAppActionParameter{
				Name:  "source_workflow",
				Value: workflowExecution.Workflow.ID,
			})

			action.Parameters = append(action.Parameters, shuffle.WorkflowAppActionParameter{
				Name:  "source_execution",
				Value: workflowExecution.ExecutionId,
			})

			//trigger.LargeImage = ""
			//err = handleSubworkflowExecution(client, workflowExecution, trigger, action)
			//if err != nil {
			//	log.Printf("[ERROR] Failed to execute subworkflow: %s", err)
			//} else {
			//	log.Printf("[INFO] Executed subworkflow!")
			//}
			//continue
		} else if action.AppName == "User Input" {
			log.Printf("USER INPUT!")

			if action.ID == workflowExecution.Start {
				log.Printf("Skipping because it's the startnode")
				visited = append(visited, action.ID)
				executed = append(executed, action.ID)
				continue
			} else {
				log.Printf("Should stop after this iteration because it's user-input based. %#v", action)
				trigger := shuffle.Trigger{}
				for _, innertrigger := range workflowExecution.Workflow.Triggers {
					if innertrigger.ID == action.ID {
						trigger = innertrigger
						break
					}
				}

				trigger.LargeImage = ""
				triggerData, err := json.Marshal(trigger)
				if err != nil {
					log.Printf("Failed unmarshalling action: %s", err)
					triggerData = []byte("Failed unmarshalling. Cancel execution!")
				}

				err = runUserInput(topClient, action, workflowExecution.Workflow.ID, workflowExecution.ExecutionId, workflowExecution.Authorization, string(triggerData))
				if err != nil {
					log.Printf("Failed launching backend magic: %s", err)
					os.Exit(3)
				} else {
					log.Printf("Launched user input node succesfully!")
					os.Exit(3)
				}

				break
			}
		} else {
			//log.Printf("Handling action %#v", action)
		}

		if len(toRemove) > 0 {
			//toRemove = []int{}
			//for index, nextAction := range nextActions {
		}

		// Not really sure how this edgecase happens.

		// FIXME
		// Execute, as we don't really care if env is not set? IDK
		if action.Environment != environment { //&& action.Environment != "" {
			//log.Printf("Action: %#v", action)
			log.Printf("Bad environment for node: %s. Want %s", action.Environment, environment)
			continue
		}

		// check whether the parent is finished executing
		//log.Printf("%s has %d parents", nextAction, len(parents[nextAction]))

		continueOuter := true
		if action.IsStartNode {
			continueOuter = false
		} else if len(parents[nextAction]) > 0 {
			// FIXME - wait for parents to finishe executing
			fixed := 0
			for _, parent := range parents[nextAction] {
				parentResult := getResult(workflowExecution, parent)
				if parentResult.Status == "FINISHED" || parentResult.Status == "SUCCESS" || parentResult.Status == "SKIPPED" || parentResult.Status == "FAILURE" {
					fixed += 1
				}
			}

			if fixed == len(parents[nextAction]) {
				continueOuter = false
			}
		} else {
			continueOuter = false
		}

		if continueOuter {
			log.Printf("[INFO] Parents of %s aren't finished: %s", nextAction, strings.Join(parents[nextAction], ", "))
			//for _, tmpaction := range parents[nextAction] {
			//	action := getAction(workflowExecution, tmpaction)
			//	_ = action
			//	//log.Printf("Parent: %s", action.Label)
			//}
			// Find the result of the nodes?
			continue
		}

		// get action status
		actionResult := getResult(workflowExecution, nextAction)
		if actionResult.Action.ID == action.ID {
			log.Printf("[INFO] %s already has status %s.", action.ID, actionResult.Status)
			continue
		} else {
			log.Printf("[INFO] %s:%s has no status result yet. Should execute.", action.Name, action.ID)
		}

		appname := action.AppName
		appversion := action.AppVersion
		appname = strings.Replace(appname, ".", "-", -1)
		appversion = strings.Replace(appversion, ".", "-", -1)

		image := fmt.Sprintf("%s:%s_%s", baseimagename, strings.ToLower(action.AppName), action.AppVersion)
		if strings.Contains(image, " ") {
			image = strings.ReplaceAll(image, " ", "-")
		}

		// Added UUID to identifier just in case
		identifier := fmt.Sprintf("%s_%s_%s_%s_%s", appname, appversion, action.ID, workflowExecution.ExecutionId, uuid.NewV4())
		if strings.Contains(identifier, " ") {
			identifier = strings.ReplaceAll(identifier, " ", "-")
		}

		// FIXME - check whether it's running locally yet too
		dockercli, err := dockerclient.NewEnvClient()
		if err != nil {
			log.Printf("[ERROR] Unable to create docker client (2): %s", err)
			//return err
			return
		}

		stats, err := dockercli.ContainerInspect(context.Background(), identifier)
		if err != nil || stats.ContainerJSONBase.State.Status != "running" {
			// REMOVE
			if err == nil {
				log.Printf("Status: %s, should kill: %s", stats.ContainerJSONBase.State.Status, identifier)
				err = removeContainer(identifier)
				if err != nil {
					log.Printf("Error killing container: %s", err)
				}
			} else {
				//log.Printf("WHAT TO DO HERE?: %s", err)
			}
		} else if stats.ContainerJSONBase.State.Status == "running" {
			//log.Printf("
			continue
		}

		if len(action.Parameters) == 0 {
			action.Parameters = []shuffle.WorkflowAppActionParameter{}
		}

		if len(action.Errors) == 0 {
			action.Errors = []string{}
		}

		// marshal action and put it in there rofl
		log.Printf("[INFO] Time to execute %s (%s) with app %s:%s, function %s, env %s with %d parameters.", action.ID, action.Label, action.AppName, action.AppVersion, action.Name, action.Environment, len(action.Parameters))

		actionData, err := json.Marshal(action)
		if err != nil {
			log.Printf("Failed unmarshalling action: %s", err)
			continue
		}

		if action.AppID == "0ca8887e-b4af-4e3e-887c-87e9d3bc3d3e" {
			log.Printf("\nShould run filter: %#v\n\n", action)
			runFilter(workflowExecution, action)
			continue
		}

		executionData, err := json.Marshal(workflowExecution)
		if err != nil {
			log.Printf("Failed marshalling executiondata: %s", err)
			executionData = []byte("")
		}

		// Sending full execution so that it won't have to load in every app
		// This might be an issue if they can read environments, but that's alright
		// if everything is generated during execution
		log.Printf("[INFO] Deployed with CALLBACK_URL %s and BASE_URL %s", appCallbackUrl, baseUrl)
		env := []string{
			fmt.Sprintf("ACTION=%s", string(actionData)),
			fmt.Sprintf("EXECUTIONID=%s", workflowExecution.ExecutionId),
			fmt.Sprintf("AUTHORIZATION=%s", workflowExecution.Authorization),
			fmt.Sprintf("CALLBACK_URL=%s", baseUrl),
			fmt.Sprintf("BASE_URL=%s", appCallbackUrl),
		}

		// Fixes issue:
		// standard_init_linux.go:185: exec user process caused "argument list too long"
		// https://devblogs.microsoft.com/oldnewthing/20100203-00/?p=15083
		maxSize := 32700 - len(string(actionData)) - 2000
		if len(executionData) < maxSize {
			log.Printf("[INFO] ADDING FULL_EXECUTION because size is smaller than %d", maxSize)
			env = append(env, fmt.Sprintf("FULL_EXECUTION=%s", string(executionData)))
		} else {
			log.Printf("[WARNING] Skipping FULL_EXECUTION because size is larger than %d", maxSize)
		}

		// Uses a few ways of getting / checking if an app is available
		// 1. Try original with lowercase
		// 2. Go to original
		// 3. Add remote repo location
		// 4. Actually download last repo

		images := []string{
			image,
			fmt.Sprintf("%s:%s_%s", baseimagename, action.AppName, action.AppVersion),
			fmt.Sprintf("%s/%s:%s_%s", registryName, baseimagename, strings.ToLower(action.AppName), action.AppVersion),
		}

		// If cleanup is set, it should run for efficiency
		pullOptions := types.ImagePullOptions{}
		if cleanupEnv == "true" {
			err = deployApp(dockercli, images[0], identifier, env, workflowExecution)
			if err != nil {
				if strings.Contains(err.Error(), "exited prematurely") {
					shutdown(workflowExecution, action.ID, err.Error(), true)
				}

				log.Printf("[WARNING] Failed CLEANUP execution. Downloading image remotely.")
				image = images[2]
				reader, err := dockercli.ImagePull(context.Background(), image, pullOptions)
				if err != nil {
					log.Printf("[ERROR] Failed getting %s. The couldn't be find locally, AND is missing.", image)
					shutdown(workflowExecution, action.ID, err.Error(), true)
				}

				buildBuf := new(strings.Builder)
				_, err = io.Copy(buildBuf, reader)
				if err != nil {
					log.Printf("[ERROR] Error in IO copy: %s", err)
					shutdown(workflowExecution, action.ID, err.Error(), true)
				} else {
					if strings.Contains(buildBuf.String(), "errorDetail") {
						log.Printf("[ERROR] Docker build:\n%s\nERROR ABOVE: Trying to pull tags from: %s", buildBuf.String(), image)
						shutdown(workflowExecution, action.ID, err.Error(), true)
					}

					log.Printf("[INFO] Successfully downloaded %s", image)
				}

				err = deployApp(dockercli, image, identifier, env, workflowExecution)
				if err != nil {

					log.Printf("[ERROR] Failed deploying image for the FOURTH time. Aborting if the image doesn't exist")
					if strings.Contains(err.Error(), "exited prematurely") {
						shutdown(workflowExecution, action.ID, err.Error(), true)
					}

					if strings.Contains(err.Error(), "No such image") {
						//log.Printf("[WARNING] Failed deploying %s from image %s: %s", identifier, image, err)
						log.Printf("[ERROR] Image doesn't exist. Shutting down")
						shutdown(workflowExecution, action.ID, err.Error(), true)
					}
				}
			}
		} else {

			err = deployApp(dockercli, images[0], identifier, env, workflowExecution)
			if err != nil {
				if strings.Contains(err.Error(), "exited prematurely") {
					shutdown(workflowExecution, action.ID, err.Error(), true)
				}

				// Trying to replace with lowercase to deploy again. This seems to work with Dockerhub well.
				// FIXME: Should try to remotely download directly if this persists.
				image = images[1]
				if strings.Contains(image, " ") {
					image = strings.ReplaceAll(image, " ", "-")
				}

				err = deployApp(dockercli, image, identifier, env, workflowExecution)
				if err != nil {
					if strings.Contains(err.Error(), "exited prematurely") {
						shutdown(workflowExecution, action.ID, err.Error(), true)
					}

					image = images[2]
					if strings.Contains(image, " ") {
						image = strings.ReplaceAll(image, " ", "-")
					}

					err = deployApp(dockercli, image, identifier, env, workflowExecution)
					if err != nil {
						if strings.Contains(err.Error(), "exited prematurely") {
							shutdown(workflowExecution, action.ID, err.Error(), true)
						}

						log.Printf("[WARNING] Failed deploying image THREE TIMES. Attempting to download the latter as last resort.")
						reader, err := dockercli.ImagePull(context.Background(), image, pullOptions)
						if err != nil {
							log.Printf("[ERROR] Failed getting %s. The couldn't be find locally, AND is missing.", image)
							shutdown(workflowExecution, action.ID, err.Error(), true)
						}

						buildBuf := new(strings.Builder)
						_, err = io.Copy(buildBuf, reader)
						if err != nil {
							log.Printf("[ERROR] Error in IO copy: %s", err)
							shutdown(workflowExecution, action.ID, err.Error(), true)
						} else {
							if strings.Contains(buildBuf.String(), "errorDetail") {
								log.Printf("[ERROR] Docker build:\n%s\nERROR ABOVE: Trying to pull tags from: %s", buildBuf.String(), image)
								shutdown(workflowExecution, action.ID, err.Error(), true)
							}

							log.Printf("[INFO] Successfully downloaded %s", image)
						}

						err = deployApp(dockercli, image, identifier, env, workflowExecution)
						if err != nil {
							log.Printf("[ERROR] Failed deploying image for the FOURTH time. Aborting if the image doesn't exist")
							if strings.Contains(err.Error(), "exited prematurely") {
								shutdown(workflowExecution, action.ID, err.Error(), true)
							}

							if strings.Contains(err.Error(), "No such image") {
								//log.Printf("[WARNING] Failed deploying %s from image %s: %s", identifier, image, err)
								log.Printf("[ERROR] Image doesn't exist. Shutting down")
								shutdown(workflowExecution, action.ID, err.Error(), true)
							}
						}
					}
				}
			}
		}

		log.Printf("[INFO] Adding visited (3): %s", action.Label)

		visited = append(visited, action.ID)
		executed = append(executed, action.ID)

		// If children of action.ID are NOT in executed:
		// Remove them from visited.
		//log.Printf("EXECUTED: %#v", executed)
	}

	//log.Println(nextAction)
	//log.Println(startAction, children[startAction])

	// FIXME - new request here
	// FIXME - clean up stopped (remove) containers with this execution id

	if len(workflowExecution.Results) == len(workflowExecution.Workflow.Actions)+extra {
		shutdownCheck := true
		for _, result := range workflowExecution.Results {
			if result.Status == "EXECUTING" {
				// Cleaning up executing stuff
				shutdownCheck = false
				// USED TO BE CONTAINER REMOVAL
				//  FIXME - send POST request to kill the container
				//log.Printf("Should remove (POST request) stopped containers")
				//ret = requests.post("%s%s" % (self.url, stream_path), headers=headers, json=action_result)
			}
		}

		if shutdownCheck {
			log.Println("[INFO] BREAKING BECAUSE RESULTS IS SAME LENGTH AS ACTIONS. SHOULD CHECK ALL RESULTS FOR WHETHER THEY'RE DONE")
			validateFinished(workflowExecution)
			shutdown(workflowExecution, "", "", true)
		}
	}

	time.Sleep(time.Duration(sleepTime) * time.Second)
	return
}

func executionInit(workflowExecution shuffle.WorkflowExecution) error {
	parents = map[string][]string{}
	children = map[string][]string{}

	startAction = workflowExecution.Start
	log.Printf("[INFO] STARTACTION: %s", startAction)
	if len(startAction) == 0 {
		log.Printf("[INFO] Didn't find execution start action. Setting it to workflow start action.")
		startAction = workflowExecution.Workflow.Start
	}

	// Setting up extra counter
	for _, trigger := range workflowExecution.Workflow.Triggers {
		//log.Printf("Appname trigger (0): %s", trigger.AppName)
		if trigger.AppName == "User Input" || trigger.AppName == "Shuffle Workflow" {
			extra += 1
		}
	}

	nextActions = append(nextActions, startAction)
	for _, branch := range workflowExecution.Workflow.Branches {
		// Check what the parent is first. If it's trigger - skip
		sourceFound := false
		destinationFound := false
		for _, action := range workflowExecution.Workflow.Actions {
			if action.ID == branch.SourceID {
				sourceFound = true
			}

			if action.ID == branch.DestinationID {
				destinationFound = true
			}
		}

		for _, trigger := range workflowExecution.Workflow.Triggers {
			//log.Printf("Appname trigger (0): %s", trigger.AppName)
			if trigger.AppName == "User Input" || trigger.AppName == "Shuffle Workflow" {
				if branch.SourceID == "c9560766-3f85-4589-8324-311acd6be820" {
					log.Printf("BRANCH: %#v", branch)
				}

				if trigger.ID == branch.SourceID {
					log.Printf("[INFO] shuffle.Trigger %s is the source!", trigger.AppName)
					sourceFound = true
				} else if trigger.ID == branch.DestinationID {
					log.Printf("[INFO] shuffle.Trigger %s is the destination!", trigger.AppName)
					destinationFound = true
				}
			}
		}

		if sourceFound {
			parents[branch.DestinationID] = append(parents[branch.DestinationID], branch.SourceID)
		} else {
			log.Printf("[INFO] ID %s was not found in actions! Skipping parent. (TRIGGER?)", branch.SourceID)
		}

		if destinationFound {
			children[branch.SourceID] = append(children[branch.SourceID], branch.DestinationID)
		} else {
			log.Printf("[INFO] ID %s was not found in actions! Skipping child. (TRIGGER?)", branch.SourceID)
		}
	}

	/*
		log.Printf("\n\n\n[INFO] CHILDREN FOUND: %#v", children)
		log.Printf("[INFO] PARENTS FOUND: %#v", parents)
		log.Printf("[INFO] NEXT ACTIONS: %#v\n\n", nextActions)
	*/

	log.Printf("[INFO] shuffle.Actions: %d + Special shuffle.Triggers: %d", len(workflowExecution.Workflow.Actions), extra)
	onpremApps := []string{}
	toExecuteOnprem := []string{}
	for _, action := range workflowExecution.Workflow.Actions {
		if action.Environment != environment {
			continue
		}

		toExecuteOnprem = append(toExecuteOnprem, action.ID)
		actionName := fmt.Sprintf("%s:%s_%s", baseimagename, action.AppName, action.AppVersion)
		found := false
		for _, app := range onpremApps {
			if actionName == app {
				found = true
			}
		}

		if !found {
			onpremApps = append(onpremApps, actionName)
		}
	}

	if len(onpremApps) == 0 {
		return errors.New(fmt.Sprintf("No apps to handle onprem (%s)", environment))
	}

	pullOptions := types.ImagePullOptions{}
	_ = pullOptions
	for _, image := range onpremApps {
		log.Printf("[INFO] Image: %s", image)
		// Kind of gambling that the image exists.
		if strings.Contains(image, " ") {
			image = strings.ReplaceAll(image, " ", "-")
		}

		// FIXME: Reimplement for speed later
		// Skip to make it faster
		//reader, err := dockercli.ImagePull(context.Background(), image, pullOptions)
		//if err != nil {
		//	log.Printf("Failed getting %s. The app is missing or some other issue", image)
		//	shutdown(workflowExecution)
		//}

		////io.Copy(os.Stdout, reader)
		//_ = reader
		//log.Printf("Successfully downloaded and built %s", image)
	}

	return nil
}

func handleExecution(client *http.Client, req *http.Request, workflowExecution shuffle.WorkflowExecution) error {
	// if no onprem runs (shouldn't happen, but extra check), exit
	// if there are some, load the images ASAP for the app

	err := executionInit(workflowExecution)
	if err != nil {
		log.Printf("[INFO] Workflow setup failed: %s", workflowExecution.ExecutionId, err)
		shutdown(workflowExecution, "", "", true)
	}

	log.Printf("Startaction: %s", startAction)

	// source = parent node, dest = child node
	// parent can have more children, child can have more parents
	// Process the parents etc. How?
	for {
		handleExecutionResult(workflowExecution)

		//fullUrl := fmt.Sprintf("%s/api/v1/workflows/%s/executions/%s/abort", baseUrl, workflowExecution.Workflow.ID, workflowExecution.ExecutionId)
		fullUrl := fmt.Sprintf("%s/api/v1/streams", baseUrl)
		log.Printf("URL: %s", fullUrl)
		req, err := http.NewRequest(
			"POST",
			fullUrl,
			bytes.NewBuffer([]byte(data)),
		)

		newresp, err := topClient.Do(req)
		if err != nil {
			log.Printf("[ERROR] Failed making request: %s", err)
			time.Sleep(time.Duration(sleepTime) * time.Second)
			continue
		}

		body, err := ioutil.ReadAll(newresp.Body)
		if err != nil {
			log.Printf("[ERROR] Failed reading body: %s", err)
			time.Sleep(time.Duration(sleepTime) * time.Second)
			continue
		}

		if newresp.StatusCode != 200 {
			log.Printf("[ERROR] Bad statuscode: %d, %s", newresp.StatusCode, string(body))

			if strings.Contains(string(body), "Workflowexecution is already finished") {
				shutdown(workflowExecution, "", "", false)
			}

			time.Sleep(time.Duration(sleepTime) * time.Second)
			continue
		}

		err = json.Unmarshal(body, &workflowExecution)
		if err != nil {
			log.Printf("[ERROR] Failed workflowExecution unmarshal: %s", err)
			time.Sleep(time.Duration(sleepTime) * time.Second)
			continue
		}

		if workflowExecution.Status == "FINISHED" || workflowExecution.Status == "SUCCESS" {
			log.Printf("[INFO] Workflow %s is finished. Exiting worker.", workflowExecution.ExecutionId)
			shutdown(workflowExecution, "", "", true)
		}

		log.Printf("[INFO] Status: %s, Results: %d, actions: %d", workflowExecution.Status, len(workflowExecution.Results), len(workflowExecution.Workflow.Actions)+extra)
		if workflowExecution.Status != "EXECUTING" {
			log.Printf("[WARNING] Exiting as worker execution has status %s!", workflowExecution.Status)
			shutdown(workflowExecution, "", "", true)
		}

	}

	return nil
}

func arrayContains(visited []string, id string) bool {
	found := false
	for _, item := range visited {
		if item == id {
			found = true
		}
	}

	return found
}

func getResult(workflowExecution shuffle.WorkflowExecution, id string) shuffle.ActionResult {
	for _, actionResult := range workflowExecution.Results {
		if actionResult.Action.ID == id {
			return actionResult
		}
	}

	return shuffle.ActionResult{}
}

func getAction(workflowExecution shuffle.WorkflowExecution, id, environment string) shuffle.Action {
	for _, action := range workflowExecution.Workflow.Actions {
		if action.ID == id {
			return action
		}
	}

	for _, trigger := range workflowExecution.Workflow.Triggers {
		if trigger.ID == id {
			return shuffle.Action{
				ID:          trigger.ID,
				AppName:     trigger.AppName,
				Name:        trigger.AppName,
				Environment: environment,
				Label:       trigger.Label,
			}
			log.Printf("FOUND TRIGGER: %#v!", trigger)
		}
	}

	return shuffle.Action{}
}

func runUserInput(client *http.Client, action shuffle.Action, workflowId, workflowExecutionId, authorization string, configuration string) error {
	timeNow := time.Now().Unix()
	result := shuffle.ActionResult{
		Action:        action,
		ExecutionId:   workflowExecutionId,
		Authorization: authorization,
		Result:        configuration,
		StartedAt:     timeNow,
		CompletedAt:   0,
		Status:        "WAITING",
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return err
	}

	fullUrl := fmt.Sprintf("%s/api/v1/streams", baseUrl)
	req, err := http.NewRequest(
		"POST",
		fullUrl,
		bytes.NewBuffer([]byte(resultData)),
	)

	if err != nil {
		log.Printf("Error building test request: %s", err)
		return err
	}

	newresp, err := client.Do(req)
	if err != nil {
		log.Printf("Error running test request: %s", err)
		return err
	}

	body, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		log.Printf("Failed reading body when waiting: %s", err)
		return err
	}

	log.Printf("[INFO] User Input Body: %s", string(body))
	return nil
}

func runTestExecution(client *http.Client, workflowId, apikey string) (string, string) {
	fullUrl := fmt.Sprintf("%s/api/v1/workflows/%s/execute", baseUrl, workflowId)
	req, err := http.NewRequest(
		"GET",
		fullUrl,
		nil,
	)

	if err != nil {
		log.Printf("Error building test request: %s", err)
		return "", ""
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apikey))
	newresp, err := client.Do(req)
	if err != nil {
		log.Printf("Error running test request: %s", err)
		return "", ""
	}

	body, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		log.Printf("Failed reading body: %s", err)
		return "", ""
	}

	log.Printf("[INFO] Test Body: %s", string(body))
	var workflowExecution shuffle.WorkflowExecution
	err = json.Unmarshal(body, &workflowExecution)
	if err != nil {
		log.Printf("Failed workflowExecution unmarshal: %s", err)
		return "", ""
	}

	return workflowExecution.Authorization, workflowExecution.ExecutionId
}

func handleWorkflowQueue(resp http.ResponseWriter, request *http.Request) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println("(3) Failed reading body for workflowqueue")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	//log.Printf("Got result: %s", string(body))
	var actionResult shuffle.ActionResult
	err = json.Unmarshal(body, &actionResult)
	if err != nil {
		log.Printf("Failed shuffle.ActionResult unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	// 1. Get the shuffle.WorkflowExecution(ExecutionId) from the database
	// 2. if shuffle.ActionResult.Authentication != shuffle.WorkflowExecution.Authentication -> exit
	// 3. Add to and update actionResult in workflowExecution
	// 4. Push to db
	// IF FAIL: Set executionstatus: abort or cancel

	ctx := context.Background()
	workflowExecution, err := getWorkflowExecution(ctx, actionResult.ExecutionId)
	if err != nil {
		log.Printf("[ERROR] Failed getting execution (workflowqueue) %s: %s", actionResult.ExecutionId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting execution ID %s because it doesn't exist."}`, actionResult.ExecutionId)))
		return
	}

	if workflowExecution.Authorization != actionResult.Authorization {
		log.Printf("[INFO] Bad authorization key when updating node (workflowQueue) %s. Want: %s, Have: %s", actionResult.ExecutionId, workflowExecution.Authorization, actionResult.Authorization)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Bad authorization key"}`)))
		return
	}

	if workflowExecution.Status == "FINISHED" {
		log.Printf("Workflowexecution is already FINISHED. No further action can be taken")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Workflowexecution is already finished because of %s with status %s"}`, workflowExecution.LastNode, workflowExecution.Status)))
		return
	}

	// Not sure what's up here
	// FIXME - remove comment
	if workflowExecution.Status == "ABORTED" || workflowExecution.Status == "FAILURE" {

		if workflowExecution.Workflow.Configuration.ExitOnError {
			log.Printf("Workflowexecution already has status %s. No further action can be taken", workflowExecution.Status)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Workflowexecution is aborted because of %s with result %s and status %s"}`, workflowExecution.LastNode, workflowExecution.Result, workflowExecution.Status)))
			return
		} else {
			log.Printf("Continuing even though it's aborted.")
		}
	}

	//if actionResult.Status == "WAITING" && actionResult.Action.AppName == "User Input" {
	//	log.Printf("SHOULD WAIT A BIT AND RUN CLOUD STUFF WITH USER INPUT! WAITING!")

	//	var trigger shuffle.Trigger
	//	err = json.Unmarshal([]byte(actionResult.Result), &trigger)
	//	if err != nil {
	//		log.Printf("Failed unmarshaling actionresult for user input: %s", err)
	//		resp.WriteHeader(401)
	//		resp.Write([]byte(`{"success": false}`))
	//		return
	//	}

	//	orgId := workflowExecution.ExecutionOrg
	//	if len(workflowExecution.OrgId) == 0 && len(workflowExecution.Workflow.OrgId) > 0 {
	//		orgId = workflowExecution.Workflow.OrgId
	//	}

	//	err := handleUserInput(trigger, orgId, workflowExecution.Workflow.ID, workflowExecution.ExecutionId)
	//	if err != nil {
	//		log.Printf("Failed userinput handler: %s", err)
	//		actionResult.Result = fmt.Sprintf("Cloud error: %s", err)
	//		workflowExecution.Results = append(workflowExecution.Results, actionResult)
	//		workflowExecution.Status = "ABORTED"
	//		err = setshuffle.WorkflowExecution(ctx, *workflowExecution, true)
	//		if err != nil {
	//			log.Printf("Failed ")
	//		} else {
	//			log.Printf("Successfully set the execution to waiting.")
	//		}

	//		resp.WriteHeader(401)
	//		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Error: %s"}`, err)))
	//	} else {
	//		log.Printf("Successful userinput handler")
	//		resp.WriteHeader(200)
	//		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "CLOUD IS DONE"}`)))

	//		actionResult.Result = "Waiting for user feedback based on configuration"

	//		workflowExecution.Results = append(workflowExecution.Results, actionResult)
	//		workflowExecution.Status = actionResult.Status
	//		err = setshuffle.WorkflowExecution(ctx, *workflowExecution, true)
	//		if err != nil {
	//			log.Printf("Failed ")
	//		} else {
	//			log.Printf("Successfully set the execution to waiting.")
	//		}
	//	}

	//	return
	//}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
	runWorkflowExecutionTransaction(ctx, 0, workflowExecution.ExecutionId, actionResult, resp)

}

func findChildNodes(workflowExecution shuffle.WorkflowExecution, nodeId string) []string {
	//log.Printf("\nNODE TO FIX: %s\n\n", nodeId)
	allChildren := []string{nodeId}

	// 1. Find children of this specific node
	// 2. Find the children of those nodes etc.
	for _, branch := range workflowExecution.Workflow.Branches {
		if branch.SourceID == nodeId {
			//log.Printf("Children: %s", branch.DestinationID)
			allChildren = append(allChildren, branch.DestinationID)

			childNodes := findChildNodes(workflowExecution, branch.DestinationID)
			for _, bottomChild := range childNodes {
				found := false
				for _, topChild := range allChildren {
					if topChild == bottomChild {
						found = true
						break
					}
				}

				if !found {
					allChildren = append(allChildren, bottomChild)
				}
			}
		}
	}

	// Remove potential duplicates
	newNodes := []string{}
	for _, tmpnode := range allChildren {
		found := false
		for _, newnode := range newNodes {
			if newnode == tmpnode {
				found = true
				break
			}
		}

		if !found {
			newNodes = append(newNodes, tmpnode)
		}
	}

	return newNodes
}

// Will make sure transactions are always ran for an execution. This is recursive if it fails. Allowed to fail up to 5 times
func runWorkflowExecutionTransaction(ctx context.Context, attempts int64, workflowExecutionId string, actionResult shuffle.ActionResult, resp http.ResponseWriter) {
	//log.Printf("IN WORKFLOWEXECUTION SUB!")
	// Should start a tx for the execution here
	workflowExecution, err := getWorkflowExecution(ctx, workflowExecutionId)
	if err != nil {
		log.Printf("[ERROR] Failed getting execution cache: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting execution"}`)))
		return
	}

	log.Printf(`[INFO] Got result %s from %s`, actionResult.Status, actionResult.Action.ID)
	resultLength := len(workflowExecution.Results)
	dbSave := false
	setExecution := true
	//tx, err := dbclient.NewTransaction(ctx)
	//if err != nil {
	//	log.Printf("client.NewTransaction: %v", err)
	//	resp.WriteHeader(401)
	//	resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed creating transaction"}`)))
	//	return
	//}

	//key := datastore.NameKey("workflowexecution", workflowExecutionId, nil)
	//workflowExecution := &shuffle.WorkflowExecution{}
	//if err := tx.Get(key, workflowExecution); err != nil {
	//	log.Printf("[ERROR] tx.Get bug: %v", err)
	//	tx.Rollback()
	//	resp.WriteHeader(401)
	//	resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting the workflow key"}`)))
	//	return
	//}
	actionResult.Action = shuffle.Action{
		AppName:    actionResult.Action.AppName,
		AppVersion: actionResult.Action.AppVersion,
		Label:      actionResult.Action.Label,
		Name:       actionResult.Action.Name,
		ID:         actionResult.Action.ID,
		Parameters: actionResult.Action.Parameters,
	}

	if actionResult.Status == "ABORTED" || actionResult.Status == "FAILURE" {
		//dbSave = true

		newResults := []shuffle.ActionResult{}
		childNodes := []string{}
		if workflowExecution.Workflow.Configuration.ExitOnError {
			log.Printf("[WARNING] shuffle.Actionresult is %s for node %s in %s. Should set workflowExecution and exit all running functions", actionResult.Status, actionResult.Action.ID, workflowExecution.ExecutionId)
			workflowExecution.Status = actionResult.Status
			workflowExecution.LastNode = actionResult.Action.ID
			// Find underlying nodes and add them
		} else {
			log.Printf("[WARNING] shuffle.Actionresult is %s for node %s in %s. Continuing anyway because of workflow configuration.", actionResult.Status, actionResult.Action.ID, workflowExecution.ExecutionId)
			// Finds ALL childnodes to set them to SKIPPED
			// Remove duplicates
			//log.Printf("CHILD NODES: %d", len(childNodes))
			childNodes = findChildNodes(*workflowExecution, actionResult.Action.ID)
			for _, nodeId := range childNodes {
				if nodeId == actionResult.Action.ID {
					continue
				}

				// 1. Find the action itself
				// 2. Create an actionresult
				curAction := shuffle.Action{ID: ""}
				for _, action := range workflowExecution.Workflow.Actions {
					if action.ID == nodeId {
						curAction = action
						break
					}
				}

				if len(curAction.ID) == 0 {
					log.Printf("Couldn't find subnode %s", nodeId)
					continue
				}

				resultExists := false
				for _, result := range workflowExecution.Results {
					if result.Action.ID == curAction.ID {
						resultExists = true
						break
					}
				}

				if !resultExists {
					// Check parents are done here. Only add it IF all parents are skipped
					skipNodeAdd := false
					for _, branch := range workflowExecution.Workflow.Branches {
						if branch.DestinationID == nodeId {
							// If the branch's source node is NOT in childNodes, it's not a skipped parent
							sourceNodeFound := false
							for _, item := range childNodes {
								if item == branch.SourceID {
									sourceNodeFound = true
									break
								}
							}

							if !sourceNodeFound {
								// FIXME: Shouldn't add skip for child nodes of these nodes. Check if this node is parent of upcoming nodes.
								log.Printf("\n\n NOT setting node %s to SKIPPED", nodeId)
								skipNodeAdd = true

								if !arrayContains(visited, nodeId) && !arrayContains(executed, nodeId) {
									nextActions = append(nextActions, nodeId)
									log.Printf("SHOULD EXECUTE NODE %s. Next actions: %s", nodeId, nextActions)
								}
								break
							}
						}
					}

					if !skipNodeAdd {
						newResult := shuffle.ActionResult{
							Action:        curAction,
							ExecutionId:   actionResult.ExecutionId,
							Authorization: actionResult.Authorization,
							Result:        "Skipped because of previous node",
							StartedAt:     0,
							CompletedAt:   0,
							Status:        "SKIPPED",
						}

						newResults = append(newResults, newResult)
					} else {
						//log.Printf("\n\nNOT adding %s as skipaction - should add to execute?", nodeId)
						//var visited []string
						//var executed []string
						//var nextActions []string
					}
				}
			}
		}

		// Cleans up aborted, and always gives a result
		lastResult := ""
		// type shuffle.ActionResult struct {
		for _, result := range workflowExecution.Results {
			if actionResult.Action.ID == result.Action.ID {
				continue
			}

			if result.Status == "EXECUTING" {
				result.Status = actionResult.Status
				result.Result = "Aborted because of error in another node (2)"
			}

			if len(result.Result) > 0 {
				lastResult = result.Result
			}

			newResults = append(newResults, result)
		}

		workflowExecution.Result = lastResult
		workflowExecution.Results = newResults
	}

	// FIXME rebuild to be like this or something
	// workflowExecution/ExecutionId/Nodes/NodeId
	// Find the appropriate action
	if len(workflowExecution.Results) > 0 {
		// FIXME
		skip := false
		found := false
		outerindex := 0
		for index, item := range workflowExecution.Results {
			if item.Action.ID == actionResult.Action.ID {
				found = true

				if item.Status == actionResult.Status {
					skip = true
				}

				outerindex = index
				break
			}
		}

		if skip {
			//log.Printf("Both are %s. Skipping this node", item.Status)
		} else if found {
			// If result exists and execution variable exists, update execution value
			//log.Printf("Exec var backend: %s", workflowExecution.Results[outerindex].Action.ExecutionVariable.Name)
			// Finds potential execution arguments
			actionVarName := workflowExecution.Results[outerindex].Action.ExecutionVariable.Name
			if len(actionVarName) > 0 {
				log.Printf("EXECUTION VARIABLE LOCAL: %s", actionVarName)
				for index, execvar := range workflowExecution.ExecutionVariables {
					if execvar.Name == actionVarName {
						// Sets the value for the variable
						workflowExecution.ExecutionVariables[index].Value = actionResult.Result
						break
					}
				}
			}

			log.Printf("[INFO] Updating %s in workflow %s from %s to %s", actionResult.Action.ID, workflowExecution.ExecutionId, workflowExecution.Results[outerindex].Status, actionResult.Status)
			workflowExecution.Results[outerindex] = actionResult
		} else {
			workflowExecution.Results = append(workflowExecution.Results, actionResult)
			log.Printf("[INFO] Setting value (1) of %s in execution %s to %s. New result length: %d", actionResult.Action.ID, workflowExecution.ExecutionId, actionResult.Status, len(workflowExecution.Results))
		}
	} else {
		workflowExecution.Results = append(workflowExecution.Results, actionResult)
		log.Printf("[INFO] Setting value (2) of %s in execution %s to %s. New result length: %d", actionResult.Action.ID, workflowExecution.ExecutionId, actionResult.Status, len(workflowExecution.Results))
	}

	if actionResult.Status == "SKIPPED" {
		log.Printf("\n\n[INFO] Handling special case for SKIPPED!\n\n")
		childNodes := findChildNodes(*workflowExecution, actionResult.Action.ID)
		for _, nodeId := range childNodes {
			if nodeId == actionResult.Action.ID {
				continue
			}

			// 1. Find the action itself
			// 2. Create an actionresult
			curAction := shuffle.Action{ID: ""}
			for _, action := range workflowExecution.Workflow.Actions {
				if action.ID == nodeId {
					curAction = action
					break
				}
			}

			if len(curAction.ID) == 0 {
				log.Printf("Couldn't find subnode %s", nodeId)
				continue
			}

			resultExists := false
			for _, result := range workflowExecution.Results {
				if result.Action.ID == curAction.ID {
					resultExists = true
					break
				}
			}

			if !resultExists {
				// Check parents are done here. Only add it IF all parents are skipped
				skipNodeAdd := false
				for _, branch := range workflowExecution.Workflow.Branches {
					if branch.DestinationID == nodeId {
						// If the branch's source node is NOT in childNodes, it's not a skipped parent
						sourceNodeFound := false
						for _, item := range childNodes {
							if item == branch.SourceID {
								sourceNodeFound = true
								break
							}
						}

						if !sourceNodeFound {
							log.Printf("[INFO] Not setting node %s to SKIPPED", nodeId)
							skipNodeAdd = true
							break
						}
					}
				}

				if !skipNodeAdd {
					newAction := shuffle.Action{
						AppName:    curAction.AppName,
						AppVersion: curAction.AppVersion,
						Label:      curAction.Label,
						Name:       curAction.Name,
						ID:         curAction.ID,
					}

					newResult := shuffle.ActionResult{
						Action:        newAction,
						ExecutionId:   actionResult.ExecutionId,
						Authorization: actionResult.Authorization,
						Result:        "Skipped because of previous node",
						StartedAt:     0,
						CompletedAt:   0,
						Status:        "SKIPPED",
					}

					workflowExecution.Results = append(workflowExecution.Results, newResult)
				}
			}
		}
	}

	// FIXME: Have a check for skippednodes and their parents
	/*
		for resultIndex, result := range workflowExecution.Results {
			if result.Status != "SKIPPED" {
				continue
			}

			// Checks if all parents are skipped or failed.
			// Otherwise removes them from the results
			for _, branch := range workflowExecution.Workflow.Branches {
				if branch.DestinationID == result.Action.ID {
					for _, subresult := range workflowExecution.Results {
						if subresult.Action.ID == branch.SourceID {
							if subresult.Status != "SKIPPED" && subresult.Status != "FAILURE" {
								//log.Printf("SUBRESULT PARENT STATUS: %s", subresult.Status)
								//log.Printf("Should remove resultIndex: %d", resultIndex)

								// FIXME: Reinstate this?
								//workflowExecution.Results = append(workflowExecution.Results[:resultIndex], workflowExecution.Results[resultIndex+1:]...)
								_ = resultIndex

								break
							}
						}
					}
				}
			}
		}

		log.Printf("NEW LENGTH: %d", len(workflowExecution.Results))
	*/

	extraInputs := 0
	for _, trigger := range workflowExecution.Workflow.Triggers {
		if trigger.Name == "User Input" && trigger.AppName == "User Input" {
			extraInputs += 1
		} else if trigger.Name == "Shuffle Workflow" && trigger.AppName == "Shuffle Workflow" {
			extraInputs += 1
		}
	}

	//log.Printf("EXTRA: %d", extraInputs)
	//log.Printf("LENGTH: %d - %d", len(workflowExecution.Results), len(workflowExecution.Workflow.Actions)+extraInputs)

	if len(workflowExecution.Results) == len(workflowExecution.Workflow.Actions)+extraInputs {
		//log.Printf("\nIN HERE WITH RESULTS %d vs %d\n", len(workflowExecution.Results), len(workflowExecution.Workflow.Actions)+extraInputs)
		finished := true
		lastResult := ""

		// Doesn't have to be SUCCESS and FINISHED everywhere anymore.
		skippedNodes := false
		for _, result := range workflowExecution.Results {
			if result.Status == "EXECUTING" {
				finished = false
				break
			}

			// FIXME: Check if ALL parents are skipped or if its just one. Otherwise execute it
			if result.Status == "SKIPPED" {
				skippedNodes = true

				// Checks if all parents are skipped or failed. Otherwise removes them from the results
				for _, branch := range workflowExecution.Workflow.Branches {
					if branch.DestinationID == result.Action.ID {
						for _, subresult := range workflowExecution.Results {
							if subresult.Action.ID == branch.SourceID {
								if subresult.Status != "SKIPPED" && subresult.Status != "FAILURE" {
									//log.Printf("SUBRESULT PARENT STATUS: %s", subresult.Status)
									//log.Printf("Should remove resultIndex: %d", resultIndex)
									finished = false
									break
								}
							}
						}
					}

					if !finished {
						break
					}
				}
			}

			lastResult = result.Result
		}

		// FIXME: Handle skip nodes - change status?
		_ = skippedNodes

		if finished {
			dbSave = true
			log.Printf("[INFO] Execution of %s finished.", workflowExecution.ExecutionId)
			//log.Println("Might be finished based on length of results and everything being SUCCESS or FINISHED - VERIFY THIS. Setting status to finished.")

			workflowExecution.Result = lastResult
			workflowExecution.Status = "FINISHED"
			workflowExecution.CompletedAt = int64(time.Now().Unix())
			if workflowExecution.LastNode == "" {
				workflowExecution.LastNode = actionResult.Action.ID
			}

		}
	}

	// FIXME - why isn't this how it works otherwise, wtf?
	//workflow, err := getWorkflow(workflowExecution.Workflow.ID)
	//newActions := []Action{}
	//for _, action := range workflowExecution.Workflow.Actions {
	//	log.Printf("Name: %s, Env: %s", action.Name, action.Environment)
	//}

	tmpJson, err := json.Marshal(workflowExecution)
	if err == nil {
		if len(tmpJson) >= 1048487 {
			dbSave = true
			log.Printf("[ERROR] Result length is too long! Need to reduce result size")

			// Result        string `json:"result" datastore:"result,noindex"`
			// Arbitrary reduction size
			maxSize := 500000
			newResults := []shuffle.ActionResult{}
			for _, item := range workflowExecution.Results {
				if len(item.Result) > maxSize {
					item.Result = "[ERROR] Result too large to handle (https://github.com/frikky/shuffle/issues/171)"
				}

				newResults = append(newResults, item)
			}

			workflowExecution.Results = newResults
		}
	}

	// Validating that action results hasn't changed
	// Handled using cachhing, so actually pretty fast
	cacheKey := fmt.Sprintf("workflowexecution-%s", workflowExecution.ExecutionId)
	if value, found := requestCache.Get(cacheKey); found {
		parsedValue := value.(*shuffle.WorkflowExecution)
		if len(parsedValue.Results) > 0 && len(parsedValue.Results) != resultLength {
			setExecution = false
			if attempts > 5 {
				//log.Printf("\n\nSkipping execution input - %d vs %d. Attempts: (%d)\n\n", len(parsedValue.Results), resultLength, attempts)
			}

			attempts += 1
			if len(workflowExecution.Results) <= len(workflowExecution.Workflow.Actions) {
				runWorkflowExecutionTransaction(ctx, attempts, workflowExecutionId, actionResult, resp)
				return
			}
		}
	}

	if setExecution || workflowExecution.Status == "FINISHED" || workflowExecution.Status == "ABORTED" || workflowExecution.Status == "FAILURE" {
		err = setWorkflowExecution(ctx, *workflowExecution, dbSave)
		if err != nil {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed setting workflowexecution actionresult: %s"}`, err)))
			return
		}
	} else {
		log.Printf("[INFO] Skipping setexec with status %s", workflowExecution.Status)

		// Just in case. Should MAYBE validate finishing another time as well.
		// This fixes issues with e.g. shuffle.Action -> shuffle.Trigger -> shuffle.Action.
		handleExecutionResult(*workflowExecution)
		//validateFinished(workflowExecution)
	}

	//if newExecutions && len(nextActions) > 0 {
	//	handleExecutionResult(*workflowExecution)
	//}

	//resp.WriteHeader(200)
	//resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

func getWorkflowExecution(ctx context.Context, id string) (*shuffle.WorkflowExecution, error) {
	//log.Printf("IN GET WORKFLOW EXEC!")
	cacheKey := fmt.Sprintf("workflowexecution-%s", id)
	if value, found := requestCache.Get(cacheKey); found {
		parsedValue := value.(*shuffle.WorkflowExecution)
		//log.Printf("Found execution for id %s with %d results", parsedValue.ExecutionId, len(parsedValue.Results))

		//validateFinished(*parsedValue)
		return parsedValue, nil
	}

	return &shuffle.WorkflowExecution{}, errors.New("No workflowexecution defined yet")
}

func sendResult(workflowExecution shuffle.WorkflowExecution, data []byte) {
	fullUrl := fmt.Sprintf("%s/api/v1/streams", baseUrl)
	req, err := http.NewRequest(
		"POST",
		fullUrl,
		bytes.NewBuffer([]byte(data)),
	)

	if err != nil {
		log.Printf("[ERROR] Failed creating finishing request: %s", err)
		shutdown(workflowExecution, "", "", false)
	}

	newresp, err := topClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] Error running finishing request: %s", err)
		shutdown(workflowExecution, "", "", false)
	}

	body, err := ioutil.ReadAll(newresp.Body)
	log.Printf("[INFO] BACKEND STATUS: %d", newresp.StatusCode)
	if err != nil {
		log.Printf("[ERROR] Failed reading body: %s", err)
	} else {
		log.Printf("[INFO] NEWRESP (from backend): %s", string(body))
	}
}

func validateFinished(workflowExecution shuffle.WorkflowExecution) {
	log.Printf("[INFO] VALIDATION. Status: %s, shuffle.Actions: %d, Extra: %d, Results: %d\n", workflowExecution.Status, len(workflowExecution.Workflow.Actions), extra, len(workflowExecution.Results))

	//if len(workflowExecution.Results) == len(workflowExecution.Workflow.Actions)+extra {
	if (len(environments) == 1 && requestsSent == 0 && len(workflowExecution.Results) >= 1) || (len(workflowExecution.Results) >= len(workflowExecution.Workflow.Actions) && len(workflowExecution.Workflow.Actions) > 0) {
		requestsSent += 1
		//log.Printf("[FINISHED] Should send full result to %s", baseUrl)

		//data = fmt.Sprintf(`{"execution_id": "%s", "authorization": "%s"}`, executionId, authorization)
		data, err := json.Marshal(workflowExecution)
		if err != nil {
			log.Printf("[ERROR] Failed to unmarshal data for backend")
			shutdown(workflowExecution, "", "", true)
		}

		sendResult(workflowExecution, data)
	}
}

func handleGetStreamResults(resp http.ResponseWriter, request *http.Request) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println("Failed reading body for stream result queue")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	var actionResult shuffle.ActionResult
	err = json.Unmarshal(body, &actionResult)
	if err != nil {
		log.Printf("Failed shuffle.ActionResult unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	ctx := context.Background()
	workflowExecution, err := getWorkflowExecution(ctx, actionResult.ExecutionId)
	if err != nil {
		//log.Printf("Failed getting execution (streamresult) %s: %s", actionResult.ExecutionId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Bad authorization key or execution_id might not exist."}`)))
		return
	}

	// Authorization is done here
	if workflowExecution.Authorization != actionResult.Authorization {
		log.Printf("Bad authorization key when getting stream results %s.", actionResult.ExecutionId)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Bad authorization key or execution_id might not exist."}`)))
		return
	}

	newjson, err := json.Marshal(workflowExecution)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking workflow execution"}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(newjson)

}

func setWorkflowExecution(ctx context.Context, workflowExecution shuffle.WorkflowExecution, dbSave bool) error {
	//log.Printf("IN SET WORKFLOW EXEC!")
	//log.Printf("\n\n\nRESULT: %s\n\n\n", workflowExecution.Status)
	if len(workflowExecution.ExecutionId) == 0 {
		log.Printf("Workflowexeciton executionId can't be empty.")
		return errors.New("ExecutionId can't be empty.")
	}

	cacheKey := fmt.Sprintf("workflowexecution-%s", workflowExecution.ExecutionId)
	requestCache.Set(cacheKey, &workflowExecution, cache.DefaultExpiration)

	handleExecutionResult(workflowExecution)
	validateFinished(workflowExecution)
	if dbSave {
		shutdown(workflowExecution, "", "", false)
	}

	return nil
}

// GetLocalIP returns the non loopback local IP of the host
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func getAvailablePort() (net.Listener, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Printf("[WARNING] Failed to assign port by default. Defaulting to 5001")
		//return ":5001"
		return nil, err
	}

	return listener, nil
	//return fmt.Sprintf(":%d", port)
}

func webserverSetup(workflowExecution shuffle.WorkflowExecution) net.Listener {
	hostname := getLocalIP()

	// FIXME: This MAY not work because of speed between first
	// container being launched and port being assigned to webserver
	listener, err := getAvailablePort()
	if err != nil {
		log.Printf("Failed to created listener: %s", err)
		shutdown(workflowExecution, "", "", true)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	log.Printf("\n\nStarting webserver on port %d with hostname: %s\n\n", port, hostname)
	log.Printf("OLD HOSTNAME: %s", appCallbackUrl)
	appCallbackUrl = fmt.Sprintf("http://%s:%d", hostname, port)
	log.Printf("NEW HOSTNAME: %s", appCallbackUrl)

	return listener
}

func runWebserver(listener net.Listener) {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/streams", handleWorkflowQueue).Methods("POST")
	r.HandleFunc("/api/v1/streams/results", handleGetStreamResults).Methods("POST", "OPTIONS")
	http.Handle("/", r)

	//log.Fatal(http.ListenAndServe(port, nil))
	log.Fatal(http.Serve(listener, nil))
}

// Initial loop etc
func main() {
	log.Printf("[INFO] Setting up worker environment")
	sleepTime := 5

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
		},
	}

	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxy := os.Getenv("HTTPS_PROXY")
	if (len(httpProxy) > 0 || len(httpsProxy) > 0) && baseUrl != "http://shuffle-backend:5001" {
		client = &http.Client{}
	} else {
		if len(httpProxy) > 0 {
			log.Printf("Running with HTTP proxy %s (env: HTTP_PROXY)", httpProxy)
		}
		if len(httpsProxy) > 0 {
			log.Printf("Running with HTTPS proxy %s (env: HTTPS_PROXY)", httpsProxy)
		}
	}

	// WORKER_TESTING_WORKFLOW should be a workflow ID
	authorization := ""
	executionId := ""
	testing := os.Getenv("WORKER_TESTING_WORKFLOW")
	shuffle_apikey := os.Getenv("WORKER_TESTING_APIKEY")
	if len(testing) > 0 && len(shuffle_apikey) > 0 {
		// Execute a workflow and use that info
		log.Printf("[WARNING] Running test environment for worker by executing workflow %s", testing)
		authorization, executionId = runTestExecution(client, testing, shuffle_apikey)

		//os.Exit(3)
	} else {
		authorization = os.Getenv("AUTHORIZATION")
		executionId = os.Getenv("EXECUTIONID")
		log.Printf("[INFO] Running normal execution with auth %s and ID %s", authorization, executionId)
	}

	workflowExecution := shuffle.WorkflowExecution{
		ExecutionId: executionId,
	}
	if len(authorization) == 0 {
		log.Println("[INFO] No AUTHORIZATION key set in env")
		shutdown(workflowExecution, "", "", false)
	}

	if len(executionId) == 0 {
		log.Println("[INFO] No EXECUTIONID key set in env")
		shutdown(workflowExecution, "", "", false)
	}

	data = fmt.Sprintf(`{"execution_id": "%s", "authorization": "%s"}`, executionId, authorization)
	fullUrl := fmt.Sprintf("%s/api/v1/streams/results", baseUrl)
	req, err := http.NewRequest(
		"POST",
		fullUrl,
		bytes.NewBuffer([]byte(data)),
	)

	if err != nil {
		log.Println("[ERROR] Failed making request builder for backend")
		shutdown(workflowExecution, "", "", true)
	}
	topClient = client

	firstRequest := true
	for {
		// Because of this, it always has updated data.
		// Removed request requirement from app_sdk
		newresp, err := client.Do(req)
		if err != nil {
			log.Printf("[ERROR] Failed request: %s", err)
			time.Sleep(time.Duration(sleepTime) * time.Second)
			continue
		}

		body, err := ioutil.ReadAll(newresp.Body)
		if err != nil {
			log.Printf("[ERROR] Failed reading body: %s", err)
			time.Sleep(time.Duration(sleepTime) * time.Second)
			continue
		}

		if newresp.StatusCode != 200 {
			log.Printf("[ERROR] %s\nStatusCode (1): %d", string(body), newresp.StatusCode)
			time.Sleep(time.Duration(sleepTime) * time.Second)
			continue
		}

		err = json.Unmarshal(body, &workflowExecution)
		if err != nil {
			log.Printf("[ERROR] Failed workflowExecution unmarshal: %s", err)
			time.Sleep(time.Duration(sleepTime) * time.Second)
			continue
		}

		if firstRequest {
			firstRequest = false
			//workflowExecution.StartedAt = int64(time.Now().Unix())

			cacheKey := fmt.Sprintf("workflowexecution-%s", workflowExecution.ExecutionId)
			requestCache = cache.New(5*time.Minute, 10*time.Minute)
			requestCache.Set(cacheKey, &workflowExecution, cache.DefaultExpiration)
			for _, action := range workflowExecution.Workflow.Actions {
				found := false
				for _, environment := range environments {
					if action.Environment == environment {
						found = true
						break
					}
				}

				if !found {
					environments = append(environments, action.Environment)
				}
			}

			log.Printf("Environments: %s. 1 = webserver, 0 or >1 = default", environments)
			if len(environments) == 1 { //&& workflowExecution.ExecutionSource != "default" {
				log.Printf("[INFO] Running OPTIMIZED execution (not manual)")
				listener := webserverSetup(workflowExecution)
				err := executionInit(workflowExecution)
				if err != nil {
					log.Printf("[INFO] Workflow setup failed: %s", workflowExecution.ExecutionId, err)
					shutdown(workflowExecution, "", "", true)
				}

				go func() {
					time.Sleep(time.Duration(1))
					handleExecutionResult(workflowExecution)
				}()

				runWebserver(listener)
				//log.Printf("Before wait")
				//wg := sync.WaitGroup{}
				//wg.Add(1)
				//wg.Wait()
			} else {
				log.Printf("[INFO] Running NON-OPTIMIZED execution for type %s with %d environments", workflowExecution.ExecutionSource, len(environments))

			}

		}

		if workflowExecution.Status == "FINISHED" || workflowExecution.Status == "SUCCESS" {
			log.Printf("[INFO] Workflow %s is finished. Exiting worker.", workflowExecution.ExecutionId)
			shutdown(workflowExecution, "", "", true)
		}

		if workflowExecution.Status == "EXECUTING" || workflowExecution.Status == "RUNNING" {
			//log.Printf("Status: %s", workflowExecution.Status)
			err = handleExecution(client, req, workflowExecution)
			if err != nil {
				log.Printf("[INFO] Workflow %s is finished: %s", workflowExecution.ExecutionId, err)
				shutdown(workflowExecution, "", "", true)
			}
		} else {
			log.Printf("[INFO] Workflow %s has status %s. Exiting worker.", workflowExecution.ExecutionId, workflowExecution.Status)
			shutdown(workflowExecution, workflowExecution.Workflow.ID, "", true)
		}

		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
}

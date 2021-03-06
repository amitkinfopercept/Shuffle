package main

import (
	"github.com/frikky/shuffle-shared"

	"bufio"

	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"

	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	//"regexp"
	"strconv"
	"strings"
	"time"

	// Google cloud
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"google.golang.org/appengine/mail"

	"github.com/frikky/kin-openapi/openapi2"
	"github.com/frikky/kin-openapi/openapi2conv"
	"github.com/frikky/kin-openapi/openapi3"
	/*
		"github.com/frikky/kin-openapi/openapi2"
		"github.com/frikky/kin-openapi/openapi2conv"
		"github.com/frikky/kin-openapi/openapi3"
	*/

	"github.com/google/go-github/v28/github"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"

	// Random
	xj "github.com/basgys/goxml2json"
	newscheduler "github.com/carlescere/scheduler"
	"github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"

	// PROXY overrides
	// "gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	// githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"

	// Web
	"github.com/gorilla/mux"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	http2 "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

// This is used to handle onprem vs offprem databases etc
var gceProject = "shuffle"
var bucketName = "shuffler.appspot.com"
var baseAppPath = "/home/frikky/git/shaffuru/tmp/apps"
var baseDockerName = "frikky/shuffle"
var registryName = "registry.hub.docker.com"
var runningEnvironment = "onprem"

var syncUrl = "https://shuffler.io"

//var syncUrl = "http://localhost:5002"
var syncSubUrl = "https://shuffler.io"

//var syncUrl = "http://localhost:5002"
//var syncSubUrl = "https://050196912a9d.ngrok.io"

var dbclient *datastore.Client

type Userapi struct {
	Username string `datastore:"username"`
	ApiKey   string `datastore:"apikey"`
}

type ExecutionInfo struct {
	TotalApiUsage           int64 `json:"total_api_usage" datastore:"total_api_usage"`
	TotalWorkflowExecutions int64 `json:"total_workflow_executions" datastore:"total_workflow_executions"`
	TotalAppExecutions      int64 `json:"total_app_executions" datastore:"total_app_executions"`
	TotalCloudExecutions    int64 `json:"total_cloud_executions" datastore:"total_cloud_executions"`
	TotalOnpremExecutions   int64 `json:"total_onprem_executions" datastore:"total_onprem_executions"`
	DailyApiUsage           int64 `json:"daily_api_usage" datastore:"daily_api_usage"`
	DailyWorkflowExecutions int64 `json:"daily_workflow_executions" datastore:"daily_workflow_executions"`
	DailyAppExecutions      int64 `json:"daily_app_executions" datastore:"daily_app_executions"`
	DailyCloudExecutions    int64 `json:"daily_cloud_executions" datastore:"daily_cloud_executions"`
	DailyOnpremExecutions   int64 `json:"daily_onprem_executions" datastore:"daily_onprem_executions"`
}

type StatisticsData struct {
	Timestamp int64  `json:"timestamp" datastore:"timestamp"`
	Id        string `json:"id" datastore:"id"`
	Amount    int64  `json:"amount" datastore:"amount"`
}

type StatisticsItem struct {
	Total     int64            `json:"total" datastore:"total"`
	Fieldname string           `json:"field_name" datastore:"field_name"`
	Data      []StatisticsData `json:"data" datastore:"data"`
	OrgId     string           `json:"org_id" datastore:"org_id"`
}

// "Execution by status"
// Execution history
//type GlobalStatistics struct {
//	BackendExecutions     int64            `json:"backend_executions" datastore:"backend_executions"`
//	WorkflowCount         int64            `json:"workflow_count" datastore:"workflow_count"`
//	ExecutionCount        int64            `json:"execution_count" datastore:"execution_count"`
//	ExecutionSuccessCount int64            `json:"execution_success_count" datastore:"execution_success_count"`
//	ExecutionAbortCount   int64            `json:"execution_abort_count" datastore:"execution_abort_count"`
//	ExecutionFailureCount int64            `json:"execution_failure_count" datastore:"execution_failure_count"`
//	ExecutionPendingCount int64            `json:"execution_pending_count" datastore:"execution_pending_count"`
//	AppUsageCount         int64            `json:"app_usage_count" datastore:"app_usage_count"`
//	TotalAppsCount        int64            `json:"total_apps_count" datastore:"total_apps_count"`
//	SelfMadeAppCount      int64            `json:"self_made_app_count" datastore:"self_made_app_count"`
//	WebhookUsageCount     int64            `json:"webhook_usage_count" datastore:"webhook_usage_count"`
//	Baseline              map[string]int64 `json:"baseline" datastore:"baseline"`
//}

type ParsedOpenApi struct {
	Body    string `datastore:"body,noindex" json:"body"`
	ID      string `datastore:"id" json:"id"`
	Success bool   `datastore:"success,omitempty" json:"success,omitempty"`
}

// Limits set for a user so that they can't do a shitload
type UserLimits struct {
	DailyApiUsage           int64 `json:"daily_api_usage" datastore:"daily_api_usage"`
	DailyWorkflowExecutions int64 `json:"daily_workflow_executions" datastore:"daily_workflow_executions"`
	DailyCloudExecutions    int64 `json:"daily_cloud_executions" datastore:"daily_cloud_executions"`
	DailyTriggers           int64 `json:"daily_triggers" datastore:"daily_triggers"`
	DailyMailUsage          int64 `json:"daily_mail_usage" datastore:"daily_mail_usage"`
	MaxTriggers             int64 `json:"max_triggers" datastore:"max_triggers"`
	MaxWorkflows            int64 `json:"max_workflows" datastore:"max_workflows"`
}

type retStruct struct {
	Success         bool                 `json:"success"`
	SyncFeatures    shuffle.SyncFeatures `json:"sync_features"`
	SessionKey      string               `json:"session_key"`
	IntervalSeconds int64                `json:"interval_seconds"`
	Reason          string               `json:"reason"`
}

// Saves some data, not sure what to have here lol
type UserAuth struct {
	Description string          `json:"description" datastore:"description,noindex" yaml:"description"`
	Name        string          `json:"name" datastore:"name" yaml:"name"`
	Workflows   []string        `json:"workflows" datastore:"workflows"`
	Username    string          `json:"username" datastore:"username"`
	Fields      []UserAuthField `json:"fields" datastore:"fields"`
}

type UserAuthField struct {
	Key   string `json:"key" datastore:"key"`
	Value string `json:"value" datastore:"value,noindex"`
}

// Not environment, but execution environment
//type Environment struct {
//	Name       string `datastore:"name"`
//	Type       string `datastore:"type"`
//	Registered bool   `datastore:"registered"`
//	Default    bool   `datastore:"default" json:"default"`
//	Archived   bool   `datastore:"archived" json:"archived"`
//	Id         string `datastore:"id" json:"id"`
//	OrgId      string `datastore:"org_id" json:"org_id"`
//}

// timeout maybe? idk
type session struct {
	Username string `datastore:"Username,noindex"`
	Id       string `datastore:"Id,noindex"`
	Session  string `datastore:"session,noindex"`
}

type loginStruct struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Contact struct {
	Firstname   string `json:"firstname"`
	Lastname    string `json:"lastname"`
	Title       string `json:"title"`
	Companyname string `json:"companyname"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Message     string `json:"message"`
}

type Translator struct {
	Src struct {
		Name        string `json:"name" datastore:"name"`
		Value       string `json:"value" datastore:"value,noindex"`
		Description string `json:"description" datastore:"description,noindex"`
		Required    string `json:"required" datastore:"required"`
		Type        string `json:"type" datastore:"type"`
		Schema      struct {
			Type string `json:"type" datastore:"type"`
		} `json:"schema" datastore:"schema"`
	} `json:"src" datastore:"src"`
	Dst struct {
		Name        string `json:"name" datastore:"name"`
		Value       string `json:"value" datastore:"value,noindex"`
		Type        string `json:"type" datastore:"type"`
		Description string `json:"description" datastore:"description,noindex"`
		Required    string `json:"required" datastore:"required"`
		Schema      struct {
			Type string `json:"type" datastore:"type"`
		} `json:"schema" datastore:"schema"`
	} `json:"dst" datastore:"dst"`
}

type Appconfig struct {
	Key   string `json:"key" datastore:"key"`
	Value string `json:"value" datastore:"value,noindex"`
}

type ScheduleApp struct {
	Foldername  string      `json:"foldername" datastore:"foldername,noindex"`
	Name        string      `json:"name" datastore:"name,noindex"`
	Id          string      `json:"id" datastore:"id,noindex"`
	Description string      `json:"description" datastore:"description,noindex"`
	Action      string      `json:"action" datastore:"action,noindex"`
	Config      []Appconfig `json:"config,omitempty" datastore:"config,noindex"`
}

type AppInfo struct {
	SourceApp      ScheduleApp `json:"sourceapp,omitempty" datastore:"sourceapp,noindex"`
	DestinationApp ScheduleApp `json:"destinationapp,omitempty" datastore:"destinationapp,noindex"`
}

// May 2020: Reused for onprem schedules - Id, Seconds, WorkflowId and argument
type ScheduleOld struct {
	Id                   string       `json:"id" datastore:"id"`
	StartNode            string       `json:"start_node" datastore:"start_node"`
	Seconds              int          `json:"seconds" datastore:"seconds"`
	WorkflowId           string       `json:"workflow_id" datastore:"workflow_id", `
	Argument             string       `json:"argument" datastore:"argument"`
	WrappedArgument      string       `json:"wrapped_argument" datastore:"wrapped_argument"`
	AppInfo              AppInfo      `json:"appinfo" datastore:"appinfo,noindex"`
	Finished             bool         `json:"finished" finished:"id"`
	BaseAppLocation      string       `json:"base_app_location" datastore:"baseapplocation,noindex"`
	Translator           []Translator `json:"translator,omitempty" datastore:"translator"`
	Org                  string       `json:"org" datastore:"org"`
	CreatedBy            string       `json:"createdby" datastore:"createdby"`
	Availability         string       `json:"availability" datastore:"availability"`
	CreationTime         int64        `json:"creationtime" datastore:"creationtime,noindex"`
	LastModificationtime int64        `json:"lastmodificationtime" datastore:"lastmodificationtime,noindex"`
	LastRuntime          int64        `json:"lastruntime" datastore:"lastruntime,noindex"`
	Frequency            string       `json:"frequency" datastore:"frequency,noindex"`
	Environment          string       `json:"environment" datastore:"environment"`
}

// Returned from /GET /schedules
type Schedules struct {
	Schedules []ScheduleOld `json:"schedules"`
	Success   bool          `json:"success"`
}

type ScheduleApps struct {
	Apps    []ApiYaml `json:"apps"`
	Success bool      `json:"success"`
}

// The yaml that is uploaded
type ApiYaml struct {
	Name        string `json:"name" yaml:"name" required:"true datastore:"name"`
	Foldername  string `json:"foldername" yaml:"foldername" required:"true datastore:"foldername"`
	Id          string `json:"id" yaml:"id",required:"true, datastore:"id"`
	Description string `json:"description" datastore:"description,noindex" yaml:"description"`
	AppVersion  string `json:"app_version" yaml:"app_version",datastore:"app_version"`
	ContactInfo struct {
		Name string `json:"name" datastore:"name" yaml:"name"`
		Url  string `json:"url" datastore:"url" yaml:"url"`
	} `json:"contact_info" datastore:"contact_info" yaml:"contact_info"`
	Types []string `json:"types" datastore:"types" yaml:"types"`
	Input []struct {
		Name            string `json:"name" datastore:"name" yaml:"name"`
		Description     string `json:"description" datastore:"description,noindex" yaml:"description"`
		InputParameters []struct {
			Name        string `json:"name" datastore:"name" yaml:"name"`
			Description string `json:"description" datastore:"description,noindex" yaml:"description"`
			Required    string `json:"required" datastore:"required" yaml:"required"`
			Schema      struct {
				Type string `json:"type" datastore:"type" yaml:"type"`
			} `json:"schema" datastore:"schema" yaml:"schema"`
		} `json:"inputparameters" datastore:"inputparameters" yaml:"inputparameters"`
		OutputParameters []struct {
			Name        string `json:"name" datastore:"name" yaml:"name"`
			Description string `json:"description" datastore:"description,noindex" yaml:"description"`
			Required    string `json:"required" datastore:"required" yaml:"required"`
			Schema      struct {
				Type string `json:"type" datastore:"type" yaml:"type"`
			} `json:"schema" datastore:"schema" yaml:"schema"`
		} `json:"outputparameters" datastore:"outputparameters" yaml:"outputparameters"`
		Config []struct {
			Name        string `json:"name" datastore:"name" yaml:"name"`
			Description string `json:"description" datastore:"description,noindex" yaml:"description"`
			Required    string `json:"required" datastore:"required" yaml:"required"`
			Schema      struct {
				Type string `json:"type" datastore:"type" yaml:"type"`
			} `json:"schema" datastore:"schema" yaml:"schema"`
		} `json:"config" datastore:"config" yaml:"config"`
	} `json:"input" datastore:"input" yaml:"input"`
	Output []struct {
		Name        string `json:"name" datastore:"name" yaml:"name"`
		Description string `json:"description" datastore:"description,noindex" yaml:"description"`
		Config      []struct {
			Name        string `json:"name" datastore:"name" yaml:"name"`
			Description string `json:"description" datastore:"description,noindex" yaml:"description"`
			Required    string `json:"required" datastore:"required" yaml:"required"`
			Schema      struct {
				Type string `json:"type" datastore:"type" yaml:"type"`
			} `json:"schema" datastore:"schema" yaml:"schema"`
		} `json:"config" datastore:"config" yaml:"config"`
		InputParameters []struct {
			Name        string `json:"name" datastore:"name" yaml:"name"`
			Description string `json:"description" datastore:"description,noindex" yaml:"description"`
			Required    string `json:"required" datastore:"required" yaml:"required"`
			Schema      struct {
				Type string `json:"type" datastore:"type" yaml:"type"`
			} `json:"schema" datastore:"schema" yaml:"schema"`
		} `json:"inputparameters" datastore:"inputparameters" yaml:"inputparameters"`
		OutputParameters []struct {
			Name        string `json:"name" datastore:"name" yaml:"name"`
			Description string `json:"description" datastore:"description,noindex" yaml:"description"`
			Required    string `json:"required" datastore:"required" yaml:"required"`
			Schema      struct {
				Type string `json:"type" datastore:"type" yaml:"type"`
			} `json:"schema" datastore:"schema" yaml:"schema"`
		} `json:"outputparameters" datastore:"outputparameters" yaml:"outputparameters"`
	} `json:"output" datastore:"output" yaml:"output"`
}

type Hooks struct {
	Hooks   []Hook `json:"hooks"`
	Success bool   `json:"-"`
}

type Info struct {
	Url         string `json:"url" datastore:"url"`
	Name        string `json:"name" datastore:"name"`
	Description string `json:"description" datastore:"description,noindex"`
}

// Actions to be done by webhooks etc
// Field is the actual field to use from json
type HookAction struct {
	Type  string `json:"type" datastore:"type"`
	Name  string `json:"name" datastore:"name"`
	Id    string `json:"id" datastore:"id"`
	Field string `json:"field" datastore:"field"`
}

type Hook struct {
	Id          string       `json:"id" datastore:"id"`
	Start       string       `json:"start" datastore:"start"`
	Info        Info         `json:"info" datastore:"info"`
	Actions     []HookAction `json:"actions" datastore:"actions,noindex"`
	Type        string       `json:"type" datastore:"type"`
	Owner       string       `json:"owner" datastore:"owner"`
	Status      string       `json:"status" datastore:"status"`
	Workflows   []string     `json:"workflows" datastore:"workflows"`
	Running     bool         `json:"running" datastore:"running"`
	OrgId       string       `json:"org_id" datastore:"org_id"`
	Environment string       `json:"environment" datastore:"environment"`
}

func createFileFromFile(ctx context.Context, bucket *storage.BucketHandle, remotePath, localPath string) error {
	// [START upload_file]
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	wc := bucket.Object(remotePath).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	// [END upload_file]
	return nil
}

func createFileFromBytes(ctx context.Context, bucket *storage.BucketHandle, remotePath string, data []byte) error {
	wc := bucket.Object(remotePath).NewWriter(ctx)

	byteReader := bytes.NewReader(data)
	if _, err := io.Copy(wc, byteReader); err != nil {
		return err
	}

	if err := wc.Close(); err != nil {
		return err
	}

	// [END upload_file]
	return nil
}

func deleteFile(ctx context.Context, bucket *storage.BucketHandle, remotePath string) error {

	// [START delete_file]
	o := bucket.Object(remotePath)
	if err := o.Delete(ctx); err != nil {
		return err
	}
	// [END delete_file]
	return nil
}

func readFile(ctx context.Context, bucket *storage.BucketHandle, object string) ([]byte, error) {
	// [START download_file]
	rc, err := bucket.Object(object).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return data, nil
	// [END download_file]
}

func IndexHandler(entrypoint string) func(w http.ResponseWriter, r *http.Request) {
	fn := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, entrypoint)
	}

	return http.HandlerFunc(fn)
}

func GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"id": "12345",
		"ts": time.Now().Format(time.RFC3339),
	}

	b, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	w.Write(b)
}

func jsonPrettyPrint(in string) string {
	var out bytes.Buffer
	err := json.Indent(&out, []byte(in), "", "\t")
	if err != nil {
		return in
	}
	return out.String()
}

// Does User exist?
// Does User have permission to view / run this?
// Encoding: /json?
// General authentication
func authenticate(request *http.Request) bool {
	authField := "authorization"
	authenticationKey := "topkek"
	//authFound := false

	// This should work right?
	for name, headers := range request.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			if name == authField && h == authenticationKey {
				//log.Printf("%v: %v", name, h)
				return true
			}
		}
	}

	return false
}

func publishPubsub(ctx context.Context, topic string, data []byte, attributes map[string]string) error {
	client, err := pubsub.NewClient(ctx, gceProject)
	if err != nil {
		return err
	}

	t := client.Topic(topic)
	result := t.Publish(ctx, &pubsub.Message{
		Data:       data,
		Attributes: attributes,
	})
	// Block until the result is returned and a server-generated
	// ID is returned for the published message.
	id, err := result.Get(ctx)
	if err != nil {
		return err
	}

	log.Printf("Published message for topic %s; msg ID: %v\n", topic, id)

	return nil
}

func checkError(cmdName string, cmdArgs []string) error {
	cmd := exec.Command(cmdName, cmdArgs...)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd", err)
		return err
	}

	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			fmt.Printf("Out: %s\n", scanner.Text())
		}
	}()

	err = cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error starting Cmd", err)
		return err
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error waiting for Cmd", err)
		return err
	}

	return nil
}

func md5sum(data []byte) string {
	hasher := md5.New()
	hasher.Write(data)
	newmd5 := hex.EncodeToString(hasher.Sum(nil))
	return newmd5
}

func md5sumfile(filepath string) string {
	dat, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Printf("Error in dat: %s", err)
	}

	hasher := md5.New()
	hasher.Write(dat)
	newmd5 := hex.EncodeToString(hasher.Sum(nil))

	log.Printf("%s: %s", filepath, newmd5)
	return newmd5
}

func checkFileExistsLocal(basepath string, filepath string) bool {
	User := "test"
	// md5sum
	// get tmp/results/md5sum/folder/results.json
	// parse /tmp/results/md5sum/results.json
	path := fmt.Sprintf("%s/%s", basepath, md5sumfile(filepath))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		//log.Printf("File error for %s: %s", filepath, err)
		return false
	}

	log.Printf("File %s exists. Getting for User %s.", filepath, User)
	return true
}

func handleGetallSchedules(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	var err error
	var limit = 50

	// FIXME - add org search and public / private
	key, ok := request.URL.Query()["limit"]
	if ok {
		limit, err = strconv.Atoi(key[0])
		if err != nil {
			limit = 50
		}
	}

	// Max datastore limit
	if limit > 1000 {
		limit = 1000
	}

	// Get URLs from a database index (mapped by orborus)
	ctx := context.Background()
	q := datastore.NewQuery("schedules").Limit(limit)
	var allschedules Schedules

	_, err = dbclient.GetAll(ctx, q, &allschedules.Schedules)
	if err != nil {
		log.Println(err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting schedules"}`)))
		return
	}

	newjson, err := json.Marshal(allschedules)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking"}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(newjson)
}

func redirect(w http.ResponseWriter, req *http.Request) {
	// remove/add not default ports from req.Host
	target := "https://" + req.Host + req.URL.Path
	if len(req.URL.RawQuery) > 0 {
		target += "?" + req.URL.RawQuery
	}
	log.Printf("redirect to: %s", target)
	http.Redirect(w, req, target,
		// see @andreiavrammsd comment: often 307 > 301
		http.StatusTemporaryRedirect)
}

func parseLoginParameters(resp http.ResponseWriter, request *http.Request) (loginStruct, error) {

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return loginStruct{}, err
	}

	var t loginStruct

	err = json.Unmarshal(body, &t)
	if err != nil {
		return loginStruct{}, err
	}

	return t, nil
}

// No more emails :)
func checkUsername(Username string) error {
	// Stupid first check of email loool
	//if !strings.Contains(Username, "@") || !strings.Contains(Username, ".") {
	//	return errors.New("Invalid Username")
	//}

	if len(Username) < 3 {
		return errors.New("Minimum Username length is 3")
	}

	return nil
}

func handleRegisterVerification(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	defaultMessage := "Successfully registered"

	var reference string
	location := strings.Split(request.URL.String(), "/")
	if len(location) <= 4 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	reference = location[4]

	if len(reference) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Id when registering verification is not valid"}`))
		return
	}

	ctx := context.Background()
	// With user, do a search for workflows with user or user's org attached
	// Only giving 200 to not give any suspicion whether they're onto an actual user or not
	q := datastore.NewQuery("Users").Filter("verification_token =", reference)
	var users []shuffle.User
	_, err := dbclient.GetAll(ctx, q, &users)
	if err != nil {
		log.Printf("Failed getting users for verification token: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, defaultMessage)))
		return
	}

	// FIXME - check reset_timeout
	if len(users) != 1 {
		log.Printf("Error - no user with verification id %s", reference)
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "%s"}`, defaultMessage)))
		return
	}

	Userdata := users[0]

	// FIXME: Not for cloud!
	Userdata.Verified = true
	err = shuffle.SetUser(ctx, &Userdata)
	if err != nil {
		log.Printf("Failed adding verification for user %s: %s", Userdata.Username, err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "%s"}`, defaultMessage)))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "%s"}`, defaultMessage)))
	log.Printf("%s SUCCESSFULLY FINISHED REGISTRATION", Userdata.Username)
}

func handleSetEnvironments(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// FIXME: Overhaul the top part.
	// Only admin can change environments, but if there are no users, anyone can make (first)
	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Can't handle set env auth"}`))
		return
	}

	if user.Role != "admin" {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Can't set environment without being admin"}`))
		return
	}

	ctx := context.Background()
	var environments []shuffle.Environment
	q := datastore.NewQuery("Environments").Filter("org_id =", user.ActiveOrg.Id)
	_, err = dbclient.GetAll(ctx, q, &environments)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Can't get environments when setting"}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println("Failed reading body")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed to read data"}`)))
		return
	}

	var newEnvironments []shuffle.Environment
	err = json.Unmarshal(body, &newEnvironments)
	if err != nil {
		log.Printf("Failed unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed to unmarshal data"}`)))
		return
	}

	if len(newEnvironments) < 1 {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "One environment is required"}`)))
		return
	}

	// Clear old data? Removed for archiving purpose. No straight deletion
	//for _, item := range environments {
	//	err = DeleteKey(ctx, "Environments", item.Name)
	//	if err != nil {
	//		resp.WriteHeader(401)
	//		resp.Write([]byte(`{"success": false, "reason": "Error cleaning up environment"}`))
	//		return
	//	}
	//}

	openEnvironments := 0
	for _, item := range newEnvironments {
		if !item.Archived {
			openEnvironments += 1
		}
	}

	if openEnvironments < 1 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Can't archived all environments"}`))
		return
	}

	for _, item := range newEnvironments {
		if item.OrgId != user.ActiveOrg.Id {
			item.OrgId = user.ActiveOrg.Id
		}

		err = setEnvironment(ctx, &item)
		if err != nil {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false, "reason": "Failed setting environment variable"}`))
			return
		}
	}

	//DeleteKey(ctx, entity string, value string) error {
	// FIXME - check which are in use
	//log.Printf("FIXME: Set new environments: %#v", newEnvironments)
	//log.Printf("DONT DELETE ONES THAT ARE IN USE")

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

func createNewUser(username, password, role, apikey string, org shuffle.Org) error {
	// Returns false if there is an issue
	// Use this for register
	err := shuffle.CheckPasswordStrength(password)
	if err != nil {
		log.Printf("Bad password strength: %s", err)
		return err
	}

	err = checkUsername(username)
	if err != nil {
		log.Printf("Bad Username strength: %s", err)
		return err
	}

	ctx := context.Background()
	q := datastore.NewQuery("Users").Filter("Username =", username)
	var users []shuffle.User
	_, err = dbclient.GetAll(ctx, q, &users)
	if err != nil {
		log.Printf("Failed getting user for registration: %s", err)
		return err
	}

	if len(users) > 0 {
		return errors.New(fmt.Sprintf("Username %s already exists", username))
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 8)
	if err != nil {
		log.Printf("Wrong password for %s: %s", username, err)
		return err
	}

	newUser := new(shuffle.User)
	newUser.Username = username
	newUser.Password = string(hashedPassword)
	newUser.Verified = false
	newUser.CreationTime = time.Now().Unix()
	newUser.Active = true
	newUser.Orgs = []string{org.Id}

	// FIXME - Remove this later
	if role == "admin" {
		newUser.Role = "admin"
		newUser.Roles = []string{"admin"}
	} else {
		newUser.Role = "user"
		newUser.Roles = []string{"user"}
	}

	newUser.ActiveOrg = org

	if len(apikey) > 0 {
		newUser.ApiKey = apikey
	}

	// set limits
	newUser.Limits.DailyApiUsage = 100
	newUser.Limits.DailyWorkflowExecutions = 1000
	newUser.Limits.DailyCloudExecutions = 100
	newUser.Limits.DailyTriggers = 20
	newUser.Limits.DailyMailUsage = 100
	newUser.Limits.MaxTriggers = 10
	newUser.Limits.MaxWorkflows = 10

	// Set base info for the user
	newUser.Executions.TotalApiUsage = 0
	newUser.Executions.TotalWorkflowExecutions = 0
	newUser.Executions.TotalAppExecutions = 0
	newUser.Executions.TotalCloudExecutions = 0
	newUser.Executions.TotalOnpremExecutions = 0
	newUser.Executions.DailyApiUsage = 0
	newUser.Executions.DailyWorkflowExecutions = 0
	newUser.Executions.DailyAppExecutions = 0
	newUser.Executions.DailyCloudExecutions = 0
	newUser.Executions.DailyOnpremExecutions = 0

	verifyToken := uuid.NewV4()
	ID := uuid.NewV4()
	newUser.Id = ID.String()
	newUser.VerificationToken = verifyToken.String()

	err = shuffle.SetUser(ctx, newUser)
	if err != nil {
		log.Printf("Error adding User %s: %s", username, err)
		return err
	}

	neworg, err := shuffle.GetOrg(ctx, org.Id)
	if err == nil {
		//neworg.Users = append(neworg.Users, *newUser)
		err = shuffle.SetOrg(ctx, *neworg, neworg.Id)
		if err != nil {
			log.Printf("Failed updating org with user %s", newUser.Username)
		} else {
			log.Printf("Successfully updated org with user %s!", newUser.Username)
		}
	}

	err = increaseStatisticsField(ctx, "successful_register", username, 1, org.Id)
	if err != nil {
		log.Printf("Failed to increase total apps loaded stats: %s", err)
	}

	return nil
}

func handleRegister(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// FIXME: Overhaul the top part.
	// Only admin can CREATE users, but if there are no users, anyone can make (first)
	count, countErr := getUserCount()
	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		if (countErr == nil && count > 0) || countErr != nil {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false, "reason": "Can't register without being admin"}`))
			return
		}
	}

	if count != 0 {
		if user.Role != "admin" {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false, "reason": "Can't register without being admin (2)"}`))
			return
		}
	}

	// Gets a struct of Username, password
	data, err := parseLoginParameters(resp, request)
	if err != nil {
		log.Printf("Invalid params: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	role := "user"
	if count == 0 {
		role = "admin"
	}

	ctx := context.Background()
	currentOrg := user.ActiveOrg
	if user.ActiveOrg.Id == "" {
		log.Printf("There's no active org for the user. Checking if there's a single one to assing it to.")

		var orgs []shuffle.Org
		q := datastore.NewQuery("Organizations")
		_, err = dbclient.GetAll(ctx, q, &orgs)
		if err == nil && len(orgs) == 1 {
			log.Printf("No org exists in auth. Setting to default (first one)")
			currentOrg = orgs[0]
		}

	}

	err = createNewUser(data.Username, data.Password, role, "", currentOrg)
	if err != nil {
		log.Printf("Failed registering user: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
	log.Printf("%s Successfully registered.", data.Username)
}

func handleCookie(request *http.Request) bool {
	c, err := request.Cookie("session_token")
	if err != nil {
		return false
	}

	if len(c.Value) == 0 {
		return false
	}

	return true
}

func handleUpdateUser(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	userInfo, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in apigen: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println("Failed reading body")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Missing field: user_id"}`)))
		return
	}

	type newUserStruct struct {
		Role     string `json:"role"`
		Username string `json:"username"`
		UserId   string `json:"user_id"`
	}

	ctx := context.Background()
	var t newUserStruct
	err = json.Unmarshal(body, &t)
	if err != nil {
		log.Printf("Failed unmarshaling userId: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unmarshaling. Missing field: user_id"}`)))
		return
	}

	// Should this role reflect the users' org access?
	// When you change org -> change user role
	if userInfo.Role != "admin" {
		log.Printf("%s tried to update user %s", userInfo.Username, t.UserId)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "You need to be admin to change other users"}`)))
		return
	}

	foundUser, err := shuffle.GetUser(ctx, t.UserId)
	if err != nil {
		log.Printf("Can't find user %s (update user): %s", t.UserId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false}`)))
		return
	}

	orgFound := false
	for _, item := range foundUser.Orgs {
		if item == userInfo.ActiveOrg.Id {
			orgFound = true
			break
		}
	}

	if !orgFound {
		log.Printf("User %s is admin, but can't edit users outside their own org.", userInfo.Id)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Can't change users outside your org."}`)))
		return
	}

	if t.Role != "admin" && t.Role != "user" {
		log.Printf("%s tried and failed to update user %s", userInfo.Username, t.UserId)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Can only change to role user and admin"}`)))
		return
	} else {
		// Same user - can't edit yourself
		if userInfo.Id == t.UserId {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Can't update the role of your own user"}`)))
			return
		}

		log.Printf("Updated user %s from %s to %s", foundUser.Username, foundUser.Role, t.Role)
		foundUser.Role = t.Role
		foundUser.Roles = []string{t.Role}
	}

	if len(t.Username) > 0 {
		q := datastore.NewQuery("Users").Filter("username =", t.Username)
		var users []shuffle.User
		_, err = dbclient.GetAll(ctx, q, &users)
		if err != nil {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false, "reason": "Failed getting users when updating user"}`))
			return
		}

		found := false
		for _, item := range users {
			if item.Username == t.Username {
				found = true
				break
			}
		}

		if found {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "User with username %s already exists"}`, t.Username)))
			return
		}

		foundUser.Username = t.Username
	}

	err = shuffle.SetUser(ctx, foundUser)
	if err != nil {
		log.Printf("Error patching user %s: %s", foundUser.Username, err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

func handleInfo(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	userInfo, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("[WARNING] Api authentication failed in handleInfo: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	ctx := context.Background()
	//session, err := getSession(ctx, userInfo.Session)
	//if err != nil {
	//	log.Printf("Session %#v doesn't exist: %s", session, err)
	//	resp.WriteHeader(401)
	//	resp.Write([]byte(`{"success": false, "reason": "No session"}`))
	//	return
	//}

	// This is a long check to see if an inactive admin can access the site
	parsedAdmin := "false"
	if userInfo.Role == "admin" {
		parsedAdmin = "true"
	}

	if !userInfo.Active {
		if userInfo.Role == "admin" {
			parsedAdmin = "true"

			ctx := context.Background()
			q := datastore.NewQuery("Users")
			var users []shuffle.User
			_, err = dbclient.GetAll(ctx, q, &users)
			if err != nil {
				resp.WriteHeader(401)
				resp.Write([]byte(`{"success": false, "reason": "Failed to get other users when verifying admin user"}`))
				return
			}

			activeFound := false
			adminFound := false
			for _, user := range users {
				if user.Id == userInfo.Id {
					continue
				}

				if user.Role != "admin" {
					continue
				}

				if user.Active {
					activeFound = true
				}

				adminFound = true
			}

			// Must ALWAYS be an active admin
			// Will return no access if another admin is active
			if !adminFound {
				log.Printf("NO OTHER ADMINS FOUND - CONTINUE!")
			} else {
				//
				if activeFound {
					log.Printf("OTHER ACTIVE ADMINS FOUND - CAN'T PASS")
					resp.WriteHeader(401)
					resp.Write([]byte(`{"success": false, "reason": "This user is locked"}`))
					return
				} else {
					log.Printf("NO OTHER ADMINS FOUND - CONTINUE!")
				}
			}
		} else {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false, "reason": "This user is locked"}`))
			return
		}
	}

	//log.Printf("%s  %s", session.Session, UserInfo.Session)
	//if session.Session != userInfo.Session {
	//	log.Printf("Session %s is not the same as %s for %s. %s", userInfo.Session, session.Session, userInfo.Username, err)
	//	resp.WriteHeader(401)
	//	resp.Write([]byte(`{"success": false, "reason": ""}`))
	//	return
	//}

	expiration := time.Now().Add(3600 * time.Second)
	http.SetCookie(resp, &http.Cookie{
		Name:    "session_token",
		Value:   userInfo.Session,
		Expires: expiration,
	})

	// Updating user info if there's something wrong
	if (len(userInfo.ActiveOrg.Name) == 0 || len(userInfo.ActiveOrg.Id) == 0) && len(userInfo.Orgs) > 0 {
		_, err := shuffle.GetOrg(ctx, userInfo.Orgs[0])
		if err != nil {
			var orgs []shuffle.Org
			q := datastore.NewQuery("Organizations")
			_, err = dbclient.GetAll(ctx, q, &orgs)
			if err == nil {
				newStringOrgs := []string{}
				newOrgs := []shuffle.Org{}
				for _, org := range orgs {
					if strings.ToLower(org.Name) == strings.ToLower(userInfo.Orgs[0]) {
						newOrgs = append(newOrgs, org)
						newStringOrgs = append(newStringOrgs, org.Id)
					}
				}

				if len(newOrgs) > 0 {
					userInfo.ActiveOrg = newOrgs[0]
					userInfo.Orgs = newStringOrgs

					err = shuffle.SetUser(ctx, &userInfo)
					if err != nil {
						log.Printf("Error patching User for activeOrg: %s", err)
					} else {
						log.Printf("Updated the users' org")
					}
				}
			} else {
				log.Printf("Failed getting orgs for user. Major issue.: %s", err)
			}

		} else {
			// 1. Check if the org exists by ID
			// 2. if it does, overwrite user
			userInfo.ActiveOrg = shuffle.Org{
				Id: userInfo.Orgs[0],
			}
			err = shuffle.SetUser(ctx, &userInfo)
			if err != nil {
				log.Printf("Error patching User for activeOrg: %s", err)
			}
		}
	}

	// FIXME: Remove this dependency by updating users' orgs when org itself is updated
	org, err := shuffle.GetOrg(ctx, userInfo.ActiveOrg.Id)
	if err == nil {
		userInfo.ActiveOrg = *org
		userInfo.ActiveOrg.Users = []shuffle.User{}
	}

	userInfo.ActiveOrg.Users = []shuffle.User{}
	userInfo.ActiveOrg.SyncConfig = shuffle.SyncConfig{}
	currentOrg, err := json.Marshal(userInfo.ActiveOrg)
	if err != nil {
		currentOrg = []byte("{}")
	}

	returnData := fmt.Sprintf(`{
	"success": true, 
	"username": "%s",
	"admin": %s, 
	"tutorials": [],
	"id": "%s",
	"orgs": [%s], 
	"active_org": %s,
	"cookies": [{"key": "session_token", "value": "%s", "expiration": %d}]
}`, userInfo.Username, parsedAdmin, userInfo.Id, currentOrg, currentOrg, userInfo.Session, expiration.Unix())

	resp.WriteHeader(200)
	resp.Write([]byte(returnData))
}

type passwordReset struct {
	Password1 string `json:"newpassword"`
	Password2 string `json:"newpassword2"`
	Reference string `json:"reference"`
}

func handlePasswordReset(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	log.Println("Handling password reset")
	defaultMessage := "Successfully handled password reset"

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println("Failed reading body")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false}`)))
		return
	}

	var t passwordReset
	err = json.Unmarshal(body, &t)
	if err != nil {
		log.Println("Failed unmarshaling")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false}`)))
		return
	}

	if t.Password1 != t.Password2 {
		resp.WriteHeader(401)
		err := "Passwords don't match"
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	if len(t.Password1) < 10 || len(t.Password2) < 10 {
		resp.WriteHeader(401)
		err := "Passwords don't match - 2"
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	ctx := context.Background()

	// With user, do a search for workflows with user or user's org attached
	// Only giving 200 to not give any suspicion whether they're onto an actual user or not
	q := datastore.NewQuery("Users").Filter("reset_reference =", t.Reference)
	var users []shuffle.User
	_, err = dbclient.GetAll(ctx, q, &users)
	if err != nil {
		log.Printf("Failed getting users: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, defaultMessage)))
		return
	}

	// FIXME - check reset_timeout
	if len(users) != 1 {
		log.Printf("Error - no user with id %s", t.Reference)
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "%s"}`, defaultMessage)))
		return
	}

	Userdata := users[0]
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(t.Password1), 8)
	if err != nil {
		log.Printf("Wrong password for %s: %s", Userdata.Username, err)
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "%s"}`, defaultMessage)))
		return
	}

	Userdata.Password = string(hashedPassword)
	Userdata.ResetTimeout = 0
	Userdata.ResetReference = ""
	err = shuffle.SetUser(ctx, &Userdata)
	if err != nil {
		log.Printf("Error adding User %s: %s", Userdata.Username, err)
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "%s"}`, defaultMessage)))
		return
	}

	// FIXME - maybe send a mail here to say that the password was changed

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "%s"}`, defaultMessage)))
}

// FIXME - forward this to emails or whatever CRM system in use
func handleContact(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	var t Contact
	err = json.Unmarshal(body, &t)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	if len(t.Email) < 3 || len(t.Message) == 0 {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Please fill a valid email and message"}`)))
		return
	}

	ctx := context.Background()
	mailContent := fmt.Sprintf("Firsname: %s\nLastname: %s\nTitle: %s\nCompanyname: %s\nPhone: %s\nEmail: %s\nMessage: %s", t.Firstname, t.Lastname, t.Title, t.Companyname, t.Phone, t.Email, t.Message)
	log.Printf("Sending contact from %s", t.Email)

	msg := &mail.Message{
		Sender:  "Shuffle <frikky@shuffler.io>",
		To:      []string{"frikky@shuffler.io"},
		Subject: "Shuffler.io - New contact form",
		Body:    mailContent,
	}

	if err := mail.Send(ctx, msg); err != nil {
		log.Printf("Couldn't send email: %v", err)
	}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true, "message": "Thanks for reaching out. We will contact you soon!"}`)))
}

func getEnvironmentCount() (int, error) {
	ctx := context.Background()
	q := datastore.NewQuery("Environments").Limit(1)
	count, err := dbclient.Count(ctx, q)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func getUserCount() (int, error) {
	ctx := context.Background()
	q := datastore.NewQuery("Users").Limit(1)
	count, err := dbclient.Count(ctx, q)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func handleGetSchedules(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in set new workflowhandler: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Role != "admin" {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Admin required"}`))
		return
	}

	ctx := context.Background()
	schedules, err := getAllSchedules(ctx, user.ActiveOrg.Id)
	if err != nil {
		log.Printf("Failed getting schedules: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Couldn't get schedules"}`))
		return
	}

	newjson, err := json.Marshal(schedules)
	if err != nil {
		log.Printf("Failed unmarshal: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking environments"}`)))
		return
	}

	//log.Printf("Existing environments: %s", string(newjson))

	resp.WriteHeader(200)
	resp.Write(newjson)
}

func checkAdminLogin(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	count, err := getUserCount()
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	if count == 0 {
		log.Printf("[WARNING] No users - redirecting for management user")
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "stay"}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "redirect"}`)))
}

func handleLogin(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// Gets a struct of Username, password
	data, err := parseLoginParameters(resp, request)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	log.Printf("[INFO] Handling login of %s", data.Username)

	err = checkUsername(data.Username)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	ctx := context.Background()
	log.Printf("[INFO] Login Username: %s", data.Username)
	q := datastore.NewQuery("Users").Filter("Username =", data.Username)
	var users []shuffle.User
	_, err = dbclient.GetAll(ctx, q, &users)
	if err != nil {
		log.Printf("Failed getting user %s", data.Username)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Username and/or password is incorrect"}`))
		return
	}

	if len(users) != 1 {
		log.Printf(`Found multiple or no users with the same username: %s: %d`, data.Username, len(users))
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Error: %d users with username %s"}`, len(users), data.Username)))
		return
	}

	Userdata := users[0]

	err = bcrypt.CompareHashAndPassword([]byte(Userdata.Password), []byte(data.Password))
	if err != nil {
		log.Printf("Password for %s is incorrect: %s", data.Username, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Username and/or password is incorrect"}`))
		return
	}

	if !Userdata.Active {
		log.Printf("%s is not active, but tried to login. Error: %v", data.Username, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "This user is deactivated"}`))
		return
	}

	// FIXME - have timeout here
	loginData := `{"success": true}`
	if len(Userdata.Session) != 0 {
		log.Println("[INFO] User session already exists - resetting it")
		expiration := time.Now().Add(3600 * time.Second)

		http.SetCookie(resp, &http.Cookie{
			Name:    "session_token",
			Value:   Userdata.Session,
			Expires: expiration,
		})

		loginData = fmt.Sprintf(`{"success": true, "cookies": [{"key": "session_token", "value": "%s", "expiration": %d}]}`, Userdata.Session, expiration.Unix())
		//log.Printf("SESSION LENGTH MORE THAN 0 IN LOGIN: %s", Userdata.Session)

		err = shuffle.SetSession(ctx, Userdata, Userdata.Session)
		if err != nil {
			log.Printf("Error adding session to database: %s", err)
		}

		resp.WriteHeader(200)
		resp.Write([]byte(loginData))
		return
	} else {
		log.Printf("[INFO] User session is empty - create one!")

		sessionToken := uuid.NewV4().String()
		expiration := time.Now().Add(3600 * time.Second)
		http.SetCookie(resp, &http.Cookie{
			Name:    "session_token",
			Value:   sessionToken,
			Expires: expiration,
		})

		// ADD TO DATABASE
		err = shuffle.SetSession(ctx, Userdata, sessionToken)
		if err != nil {
			log.Printf("Error adding session to database: %s", err)
		}

		Userdata.Session = sessionToken
		err = shuffle.SetUser(ctx, &Userdata)
		if err != nil {
			log.Printf("Failed updating user when setting session: %s", err)
			resp.WriteHeader(500)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		loginData = fmt.Sprintf(`{"success": true, "cookies": [{"key": "session_token", "value": "%s", "expiration": %d}]}`, sessionToken, expiration.Unix())
	}

	log.Printf("%s SUCCESSFULLY LOGGED IN with session %s", data.Username, Userdata.Session)

	resp.WriteHeader(200)
	resp.Write([]byte(loginData))
}

// Index = Username
func DeleteKeys(ctx context.Context, entity string, value []string) error {
	// Non indexed User data
	keys := []*datastore.Key{}
	for _, item := range value {
		keys = append(keys, datastore.NameKey(entity, item, nil))
	}

	err := dbclient.DeleteMulti(ctx, keys)
	if err != nil {
		log.Printf("Error deleting %s from %s: %s", value, entity, err)
		return err
	}

	return nil
}

// Index = Username
func DeleteKey(ctx context.Context, entity string, value string) error {
	// Non indexed User data
	key1 := datastore.NameKey(entity, value, nil)

	err := dbclient.Delete(ctx, key1)
	if err != nil {
		log.Printf("Error deleting %s from %s: %s", value, entity, err)
		return err
	}

	return nil
}

func setOpenApiDatastore(ctx context.Context, id string, data ParsedOpenApi) error {
	k := datastore.NameKey("openapi3", id, nil)
	if _, err := dbclient.Put(ctx, k, &data); err != nil {
		return err
	}

	return nil
}

func getOpenApiDatastore(ctx context.Context, id string) (ParsedOpenApi, error) {
	key := datastore.NameKey("openapi3", id, nil)
	api := &ParsedOpenApi{}
	if err := dbclient.Get(ctx, key, api); err != nil {
		return ParsedOpenApi{}, err
	}

	return *api, nil
}

func setEnvironment(ctx context.Context, data *shuffle.Environment) error {
	// clear session_token and API_token for user
	k := datastore.NameKey("Environments", strings.ToLower(data.Name), nil)

	// New struct, to not add body, author etc

	if _, err := dbclient.Put(ctx, k, data); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func fixOrgUser(ctx context.Context, org *shuffle.Org) *shuffle.Org {
	//found := false
	//for _, id := range user.Orgs {
	//	if user.ActiveOrg.Id == id {
	//		found = true
	//		break
	//	}
	//}

	//if !found {
	//	user.Orgs = append(user.Orgs, user.ActiveOrg.Id)
	//}

	//// Might be vulnerable to timing attacks.
	//for _, orgId := range user.Orgs {
	//	if len(orgId) == 0 {
	//		continue
	//	}

	//	org, err := shuffle.GetOrg(ctx, orgId)
	//	if err != nil {
	//		log.Printf("Error getting org %s", orgId)
	//		continue
	//	}

	//	orgIndex := 0
	//	userFound := false
	//	for index, orgUser := range org.Users {
	//		if orgUser.Id == user.Id {
	//			orgIndex = index
	//			userFound = true
	//			break
	//		}
	//	}

	//	if userFound {
	//		user.PrivateApps = []WorkflowApp{}
	//		user.Executions = ExecutionInfo{}
	//		user.Limits = UserLimits{}
	//		user.Authentication = []UserAuth{}

	//		org.Users[orgIndex] = *user
	//	} else {
	//		org.Users = append(org.Users, *user)
	//	}

	//	err = shuffle.SetOrg(ctx, *org, orgId)
	//	if err != nil {
	//		log.Printf("Failed setting org %s", orgId)
	//	}
	//}

	return org
}

func fixUserOrg(ctx context.Context, user *shuffle.User) *shuffle.User {
	found := false
	for _, id := range user.Orgs {
		if user.ActiveOrg.Id == id {
			found = true
			break
		}
	}

	if !found {
		user.Orgs = append(user.Orgs, user.ActiveOrg.Id)
	}

	// Might be vulnerable to timing attacks.
	for _, orgId := range user.Orgs {
		if len(orgId) == 0 {
			continue
		}

		org, err := shuffle.GetOrg(ctx, orgId)
		if err != nil {
			log.Printf("Error getting org %s", orgId)
			continue
		}

		orgIndex := 0
		userFound := false
		for index, orgUser := range org.Users {
			if orgUser.Id == user.Id {
				orgIndex = index
				userFound = true
				break
			}
		}

		if userFound {
			user.PrivateApps = []shuffle.WorkflowApp{}
			user.Executions = shuffle.ExecutionInfo{}
			user.Limits = shuffle.UserLimits{}
			user.Authentication = []shuffle.UserAuth{}

			org.Users[orgIndex] = *user
		} else {
			org.Users = append(org.Users, *user)
		}

		err = shuffle.SetOrg(ctx, *org, orgId)
		if err != nil {
			log.Printf("Failed setting org %s", orgId)
		}
	}

	return user
}

// Used for testing only. Shouldn't impact production.
func handleCors(resp http.ResponseWriter, request *http.Request) bool {
	allowedOrigins := "http://localhost:3000"
	//allowedOrigins := "http://localhost:3002"

	resp.Header().Set("Vary", "Origin")
	resp.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, remember-me, Authorization")
	resp.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, PATCH")
	resp.Header().Set("Access-Control-Allow-Credentials", "true")
	resp.Header().Set("Access-Control-Allow-Origin", allowedOrigins)

	if request.Method == "OPTIONS" {
		resp.WriteHeader(200)
		resp.Write([]byte("OK"))
		return true
	}

	return false
}

func parseWorkflowParameters(resp http.ResponseWriter, request *http.Request) (map[string]interface{}, error) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("Parsing data: %s", string(body))
	var t map[string]interface{}
	err = json.Unmarshal(body, &t)
	if err == nil {
		log.Printf("PARSED!! :)")
		return t, nil
	}

	// Translate XML to json in case of an XML blob.
	// FIXME - use Content-Type and Accept headers

	xml := strings.NewReader(string(body))
	curjson, err := xj.Convert(xml)
	if err != nil {
		return t, err
	}

	//fmt.Println(curjson.String())
	//log.Printf("Parsing json a second time: %s", string(curjson.String()))

	err = json.Unmarshal(curjson.Bytes(), &t)
	if err != nil {
		return t, nil
	}

	envelope := t["Envelope"].(map[string]interface{})
	curbody := envelope["Body"].(map[string]interface{})

	//log.Println(curbody)

	// ALWAYS handle strings only
	// FIXME - remove this and get it from config or something
	requiredField := "symptomDescription"
	_, found := SearchNested(curbody, requiredField)

	// Maxdepth
	maxiter := 5

	// Need to look for parent of the item, as that is most likely root
	if found {
		cnt := 0
		var previousDifferentItem map[string]interface{}
		var previousItem map[string]interface{}
		_ = previousItem
		for {
			if cnt == maxiter {
				break
			}

			// Already know it exists
			key, realItem, _ := SearchNestedParent(curbody, requiredField)

			// First should ALWAYS work since we already have recursion checked
			if len(previousDifferentItem) == 0 {
				previousDifferentItem = realItem.(map[string]interface{})
			}

			switch t := realItem.(type) {
			case map[string]interface{}:
				previousItem = realItem.(map[string]interface{})
				curbody = realItem.(map[string]interface{})
			default:
				// Gets here if it's not an object
				_ = t
				//log.Printf("hi %#v", previousItem)
				return previousItem, nil
			}

			_ = key
			cnt += 1
		}
	}

	//key, realItem, found = SearchNestedParent(newbody, requiredField)

	//if !found {
	//	log.Println("NOT FOUND!")
	//}

	////log.Println(realItem[requiredField].(map[string]interface{}))
	//log.Println(realItem[requiredField])
	//log.Printf("FOUND PARENT :): %s", key)

	return t, nil
}

// SearchNested searches a nested structure consisting of map[string]interface{}
// and []interface{} looking for a map with a specific key name.
// If found SearchNested returns the value associated with that key, true
func SearchNestedParent(obj interface{}, key string) (string, interface{}, bool) {
	switch t := obj.(type) {
	case map[string]interface{}:
		if v, ok := t[key]; ok {
			return "", v, ok
		}
		for k, v := range t {
			if _, ok := SearchNested(v, key); ok {
				return k, v, ok
			}
		}
	case []interface{}:
		for _, v := range t {
			if _, ok := SearchNested(v, key); ok {
				return "", v, ok
			}
		}
	}

	return "", nil, false
}

// SearchNested searches a nested structure consisting of map[string]interface{}
// and []interface{} looking for a map with a specific key name.
// If found SearchNested returns the value associated with that key, true
// If the key is not found SearchNested returns nil, false
func SearchNested(obj interface{}, key string) (interface{}, bool) {
	switch t := obj.(type) {
	case map[string]interface{}:
		if v, ok := t[key]; ok {
			return v, ok
		}
		for _, v := range t {
			if result, ok := SearchNested(v, key); ok {
				return result, ok
			}
		}
	case []interface{}:
		for _, v := range t {
			if result, ok := SearchNested(v, key); ok {
				return result, ok
			}
		}
	}
	return nil, false
}

func handleSetHook(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("[INFO] Api authentication failed in set new workflowhandler: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var workflowId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		workflowId = location[4]
	}

	if len(workflowId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	// FIXME - check basic authentication
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Error with body read: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	log.Println(jsonPrettyPrint(string(body)))

	var hook Hook
	err = json.Unmarshal(body, &hook)
	if err != nil {
		log.Printf("Failed unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Id != hook.Owner && user.Role != "admin" && user.Role != "scheduler" {
		log.Printf("Wrong user (%s) for hook %s", user.Username, hook.Id)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if hook.Id != workflowId {
		errorstring := fmt.Sprintf(`Id %s != %s`, hook.Id, workflowId)
		log.Printf("Ids not matching: %s", errorstring)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "message": "%s"}`, errorstring)))
		return
	}

	// Verifies the hook JSON. Bad verification :^)
	finished, errorstring := verifyHook(hook)
	if !finished {
		log.Printf("Error with hook: %s", errorstring)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "message": "%s"}`, errorstring)))
		return
	}

	// Get the ID to see whether it exists
	// FIXME - use return and set READONLY fields (don't allow change from User)
	ctx := context.Background()
	_, err = getHook(ctx, workflowId)
	if err != nil {
		log.Printf("Failed getting hook %s (set): %s", workflowId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "Invalid ID"}`))
		return
	}

	// Update the fields
	err = setHook(ctx, hook)
	if err != nil {
		log.Printf("Failed setting hook: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

// FIXME - some fields (e.g. status) shouldn't be writeable.. Meh
func verifyHook(hook Hook) (bool, string) {
	// required fields: Id, info.name, type, status, running
	if hook.Id == "" {
		return false, "Missing required field id"
	}

	if hook.Info.Name == "" {
		return false, "Missing required field info.name"
	}

	// Validate type stuff
	validTypes := []string{"webhook"}
	found := false
	for _, key := range validTypes {
		if hook.Type == key {
			found = true
			break
		}
	}

	if !found {
		return false, fmt.Sprintf("Field type is invalid. Allowed: %s", strings.Join(validTypes, ", "))
	}

	// WEbhook specific
	if hook.Type == "webhook" {
		if hook.Info.Url == "" {
			return false, "Missing required field info.url"
		}
	}

	if hook.Status == "" {
		return false, "Missing required field status"
	}

	validStatusFields := []string{"running", "stopped", "uninitialized"}
	found = false
	for _, key := range validStatusFields {
		if hook.Status == key {
			found = true
			break
		}
	}

	if !found {
		return false, fmt.Sprintf("Field status is invalid. Allowed: %s", strings.Join(validStatusFields, ", "))
	}

	// Verify actions
	if len(hook.Actions) > 0 {
		existingIds := []string{}
		for index, action := range hook.Actions {
			if action.Type == "" {
				return false, fmt.Sprintf("Missing required field actions.type at index %d", index)
			}

			if action.Name == "" {
				return false, fmt.Sprintf("Missing required field actions.name at index %d", index)
			}

			if action.Id == "" {
				return false, fmt.Sprintf("Missing required field actions.id at index %d", index)
			}

			// Check for duplicate IDs
			for _, actionId := range existingIds {
				if action.Id == actionId {
					return false, fmt.Sprintf("actions.id %s at index %d already exists", actionId, index)
				}
			}
			existingIds = append(existingIds, action.Id)
		}
	}

	return true, "All items set"
	//log.Printf("%#v", hook)

	//Id         string   `json:"id" datastore:"id"`
	//Info       Info     `json:"info" datastore:"info"`
	//Transforms struct{} `json:"transforms" datastore:"transforms"`
	//Actions    []HookAction `json:"actions" datastore:"actions"`
	//Type       string   `json:"type" datastore:"type"`
	//Status     string   `json:"status" datastore:"status"`
	//Running    bool     `json:"running" datastore:"running"`
}

func setSpecificSchedule(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var workflowId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		workflowId = location[4]
	}

	if len(workflowId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	// FIXME - check basic authentication
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Error with body read: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	jsonPrettyPrint(string(body))
	var schedule ScheduleOld
	err = json.Unmarshal(body, &schedule)
	if err != nil {
		log.Printf("Failed unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - check access etc
	ctx := context.Background()
	err = setSchedule(ctx, schedule)
	if err != nil {
		log.Printf("Failed setting schedule: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - get some real data?
	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
	return
}

//func GetSchedule(ctx context.Context, schedulename string) (*ScheduleOld, error) {
//	key := datastore.NameKey("schedules", strings.ToLower(schedulename), nil)
//	curUser := &ScheduleOld{}
//	if err := dbclient.Get(ctx, key, curUser); err != nil {
//		return &ScheduleOld{}, err
//	}
//
//	return curUser, nil
//}

func getSpecificWebhook(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var workflowId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		workflowId = location[4]
	}

	if len(workflowId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	ctx := context.Background()
	// FIXME: Schedule = trigger?
	schedule, err := shuffle.GetSchedule(ctx, workflowId)
	if err != nil {
		log.Printf("Failed setting schedule: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	//log.Printf("%#v", schedule.Translator[0])

	b, err := json.Marshal(schedule)
	if err != nil {
		log.Printf("Failed marshalling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - get some real data?
	resp.WriteHeader(200)
	resp.Write([]byte(b))
	return
}

// Starts a new webhook
func handleDeleteSchedule(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("[WARNING] Api authentication failed in set new workflowhandler: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME: IAM - Get workflow and check owner
	if user.Role != "admin" {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Admin required"}`))
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var workflowId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		workflowId = location[4]
	}

	if len(workflowId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	ctx := context.Background()
	err = DeleteKey(ctx, "schedules", workflowId)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "Can't delete"}`))
		return
	}

	// FIXME - remove schedule too

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true, "message": "Deleted webhook"}`))
}

// Starts a new webhook
func handleNewSchedule(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	randomValue := uuid.NewV4()
	h := md5.New()
	io.WriteString(h, randomValue.String())
	newId := strings.ToLower(fmt.Sprintf("%X", h.Sum(nil)))

	// FIXME - timestamp!
	// FIXME - applocation - cloud function?
	timeNow := int64(time.Now().Unix())
	schedule := ScheduleOld{
		Id:                   newId,
		AppInfo:              AppInfo{},
		BaseAppLocation:      "/home/frikky/git/shaffuru/tmp/apps",
		CreationTime:         timeNow,
		LastModificationtime: timeNow,
		LastRuntime:          timeNow,
	}

	ctx := context.Background()
	err := setSchedule(ctx, schedule)
	if err != nil {
		log.Printf("Failed setting hook: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	log.Println("Generating new schedule")
	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true, "message": "Created new service"}`))
}

// Does the webhook
func handleWebhookCallback(resp http.ResponseWriter, request *http.Request) {
	// 1. Get callback data
	// 2. Load the configuration
	// 3. Execute the workflow

	path := strings.Split(request.URL.String(), "/")
	if len(path) < 4 {
		resp.WriteHeader(403)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// 1. Get config with hookId
	//fmt.Sprintf("%s/api/v1/hooks/%s", callbackUrl, hookId)
	ctx := context.Background()
	location := strings.Split(request.URL.String(), "/")

	var hookId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		hookId = location[4]
	}

	// ID: webhook_<UID>
	if len(hookId) != 44 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	hookId = hookId[8:len(hookId)]

	//log.Printf("HookID: %s", hookId)
	hook, err := getHook(ctx, hookId)
	if err != nil {
		log.Printf("Failed getting hook %s (callback): %s", hookId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	//log.Printf("HOOK FOUND: %#v", hook)
	// Execute the workflow
	//executeWorkflow(resp, request)

	//resp.WriteHeader(200)
	//resp.Write([]byte(`{"success": true}`))
	if hook.Status == "stopped" {
		log.Printf("[WARNING] Not running %s because hook status is stopped", hook.Id)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "The webhook isn't running. Click start to start it"}`)))
		return
	}

	if len(hook.Workflows) == 0 {
		log.Printf("Not running because hook isn't connected to any workflows")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "No workflows are defined"}`)))
		return
	}

	if hook.Environment == "cloud" {
		log.Printf("This should trigger in the cloud. Duplicate action allowed onprem.")
	}

	type ExecutionStruct struct {
		Start             string `json:"start"`
		ExecutionSource   string `json:"execution_source"`
		ExecutionArgument string `json:"execution_argument"`
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Body data error: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	//log.Printf("BODY: %s", parsedBody)

	// This is a specific fix for MSteams and may fix other things as well
	// Scared whether it may stop other things though, but that's a future problem
	// (famous last words)
	parsedBody := string(body)
	if strings.Contains(parsedBody, "choice") {
		if strings.Count(parsedBody, `\\n`) > 2 {
			parsedBody = strings.Replace(parsedBody, `\\n`, "", -1)
		}
		if strings.Count(parsedBody, `\u0022`) > 2 {
			parsedBody = strings.Replace(parsedBody, `\u0022`, `"`, -1)
		}
		if strings.Count(parsedBody, `\\"`) > 2 {
			parsedBody = strings.Replace(parsedBody, `\\"`, `"`, -1)
		}

		if strings.Contains(parsedBody, `"extra": "{`) {
			parsedBody = strings.Replace(parsedBody, `"extra": "{`, `"extra": {`, 1)
			parsedBody = strings.Replace(parsedBody, `}"}`, `}}`, 1)
		}
	}

	//log.Printf("\n\nPARSEDBODY: %s", parsedBody)
	newBody := ExecutionStruct{
		Start:             hook.Start,
		ExecutionSource:   "webhook",
		ExecutionArgument: parsedBody,
	}

	b, err := json.Marshal(newBody)
	if err != nil {
		log.Printf("Failed newBody marshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	for _, item := range hook.Workflows {
		//log.Printf("Running webhook for workflow %s with startnode %s", item, hook.Start)
		workflow := shuffle.Workflow{
			ID: "",
		}

		//parsedBody := string(body)
		//parsedBody = strings.Replace(parsedBody, "\"", "\\\"", -1)
		//if len(parsedBody) > 0 {
		//	if string(parsedBody[0]) == `"` && string(parsedBody[len(parsedBody)-1]) == "\"" {
		//		parsedBody = parsedBody[1 : len(parsedBody)-1]
		//	}
		//}

		//bodyWrapper := fmt.Sprintf(`{"start": "%s", "execution_source": "webhook", "execution_argument": "%s"}`, hook.Start, string(parsedBody))
		//if len(hook.Start) == 0 {
		//	log.Printf("No start node for hook %s - running with workflow default.", hook.Id)
		//	bodyWrapper = string(parsedBody)
		//}

		newRequest := &http.Request{
			URL:    &url.URL{},
			Method: "POST",
			Body:   ioutil.NopCloser(bytes.NewReader(b)),
		}
		//start, startok := request.URL.Query()["start"]

		// OrgId: activeOrgs[0].Id,
		workflowExecution, executionResp, err := handleExecution(item, workflow, newRequest)
		if err == nil {
			/*
				err = increaseStatisticsField(ctx, "total_webhooks_ran", workflowExecution.Workflow.ID, 1, workflowExecution.ExecutionOrg)
				if err != nil {
					log.Printf("Failed to increase total apps loaded stats: %s", err)
				}
			*/

			resp.WriteHeader(200)
			resp.Write([]byte(fmt.Sprintf(`{"success": true, "execution_id": "%s", "authorization": "%s"}`, workflowExecution.ExecutionId, workflowExecution.Authorization)))
			return
		}

		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, executionResp)))
	}
}

func executeCloudAction(action shuffle.CloudSyncJob, apikey string) error {
	data, err := json.Marshal(action)
	if err != nil {
		log.Printf("Failed cloud webhook action marshalling: %s", err)
		return err
	}

	client := &http.Client{}
	syncUrl := fmt.Sprintf("%s/api/v1/cloud/sync/handle_action", syncUrl)
	req, err := http.NewRequest(
		"POST",
		syncUrl,
		bytes.NewBuffer(data),
	)

	req.Header.Add("Authorization", fmt.Sprintf(`Bearer %s`, apikey))
	newresp, err := client.Do(req)
	if err != nil {
		return err
	}

	respBody, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		return err
	}

	type Result struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
	}

	//log.Printf("Data: %s", string(respBody))
	responseData := Result{}
	err = json.Unmarshal(respBody, &responseData)
	if err != nil {
		return err
	}

	if !responseData.Success {
		return errors.New(fmt.Sprintf("Cloud error from Shuffler: %s", responseData.Reason))
	}

	return nil
}

func getSpecificSchedule(resp http.ResponseWriter, request *http.Request) {
	if request.Method != "GET" {
		setSpecificSchedule(resp, request)
		return
	}

	cors := handleCors(resp, request)
	if cors {
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var workflowId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		workflowId = location[4]
	}

	if len(workflowId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	ctx := context.Background()
	schedule, err := shuffle.GetSchedule(ctx, workflowId)
	if err != nil {
		log.Printf("Failed getting schedule: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	//log.Printf("%#v", schedule.Translator[0])

	b, err := json.Marshal(schedule)
	if err != nil {
		log.Printf("Failed marshalling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(b))
}

func loadYaml(fileLocation string) (ApiYaml, error) {
	apiYaml := ApiYaml{}

	yamlFile, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		log.Printf("yamlFile.Get err: %s", err)
		return ApiYaml{}, err
	}

	err = yaml.Unmarshal([]byte(yamlFile), &apiYaml)
	if err != nil {
		return ApiYaml{}, err
	}

	return apiYaml, nil
}

// This should ALWAYS come from an OUTPUT
func executeSchedule(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	location := strings.Split(request.URL.String(), "/")
	var workflowId string

	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		workflowId = location[4]
	}

	if len(workflowId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	ctx := context.Background()
	log.Printf("[INFO] EXECUTING %s!", workflowId)
	idConfig, err := shuffle.GetSchedule(ctx, workflowId)
	if err != nil {
		log.Printf("Error getting schedule: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	// Basically the src app
	inputStrings := map[string]string{}
	for _, item := range idConfig.Translator {
		if item.Dst.Required == "false" {
			log.Println("Skipping not required")
			continue
		}

		if item.Src.Name == "" {
			errorMsg := fmt.Sprintf("Required field %s has no source", item.Dst.Name)
			log.Println(errorMsg)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, errorMsg)))
			return
		}

		inputStrings[item.Dst.Name] = item.Src.Name
	}

	configmap := map[string]string{}
	for _, config := range idConfig.AppInfo.SourceApp.Config {
		configmap[config.Key] = config.Value
	}

	// FIXME - this wont work for everything lmao
	functionName := strings.ToLower(idConfig.AppInfo.SourceApp.Action)
	functionName = strings.Replace(functionName, " ", "_", 10)

	cmdArgs := []string{
		fmt.Sprintf("%s/%s/app.py", baseAppPath, "thehive"),
		fmt.Sprintf("--referenceid=%s", workflowId),
		fmt.Sprintf("--function=%s", functionName),
	}

	for key, value := range configmap {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", key, value))
	}

	// FIXME - processname
	baseProcess := "python3"
	log.Printf("Executing: %s %s", baseProcess, strings.Join(cmdArgs, " "))
	execSubprocess(baseProcess, cmdArgs)

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

func execSubprocess(cmdName string, cmdArgs []string) error {
	cmd := exec.Command(cmdName, cmdArgs...)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd", err)
		return err
	}

	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			fmt.Printf("Out: %s\n", scanner.Text())
		}
	}()

	err = cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error starting Cmd", err)
		return err
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error waiting for Cmd", err)
		return err
	}

	return nil
}

// This should ALWAYS come from an OUTPUT
func uploadWorkflowResult(resp http.ResponseWriter, request *http.Request) {
	// Post to a key with random data?
	location := strings.Split(request.URL.String(), "/")

	var workflowId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		workflowId = location[4]
	}

	if len(workflowId) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "ID not valid"}`))
		return
	}

	// FIXME - check if permission AND whether it exists

	// FIXME - validate ID as well
	ctx := context.Background()
	schedule, err := shuffle.GetSchedule(ctx, workflowId)
	if err != nil {
		log.Printf("Failed setting schedule %s: %s", workflowId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Should use generic interfaces and parse fields OR
	// build temporary struct based on api.yaml of the app
	data, err := parseWorkflowParameters(resp, request)
	if err != nil {
		log.Printf("Invalid params: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	// Get the actual fields
	foldername := schedule.AppInfo.SourceApp.Foldername
	curOutputType := schedule.AppInfo.SourceApp.Name
	curOutputAppOutput := schedule.AppInfo.SourceApp.Action
	curInputType := schedule.AppInfo.DestinationApp.Name
	translatormap := schedule.Translator

	if len(curOutputType) <= 0 {
		log.Printf("Id %s is invalid. Missing sourceapp name", workflowId)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false}`)))
		return
	}

	if len(foldername) == 0 {
		foldername = strings.ToLower(curOutputType)
	}

	if len(curOutputAppOutput) <= 0 {
		log.Printf("Id %s is invalid. Missing source output ", workflowId)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false}`)))
		return
	}

	if len(curInputType) <= 0 {
		log.Printf("Id %s is invalid. Missing destination name", workflowId)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false}`)))
		return
	}

	// Needs to be used for parsing properly
	// Might be dumb to have the yaml as a file too
	yamlpath := fmt.Sprintf("%s/%s/api.yaml", baseAppPath, foldername)
	curyaml, err := loadYaml(yamlpath)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	//validFields := []string{}
	requiredFields := []string{}
	optionalFields := []string{}
	for _, output := range curyaml.Output {
		if output.Name != curOutputAppOutput {
			continue
		}

		for _, outputparam := range output.OutputParameters {
			if outputparam.Required == "true" {
				if outputparam.Schema.Type == "string" {
					requiredFields = append(requiredFields, outputparam.Name)
				} else {
					log.Printf("Outputparam schematype %s is not implemented.", outputparam.Schema.Type)
				}
			} else {
				optionalFields = append(optionalFields, outputparam.Name)
			}
		}

		// Wont reach here unless it's the right one
		break
	}

	// Checks whether ALL required fields are filled
	for _, fieldname := range requiredFields {
		if data[fieldname] == nil {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Field %s is required"}`, fieldname)))
			return
		} else {
			log.Printf("%s: %s", fieldname, data[fieldname])
		}
	}

	// FIXME
	// Verify whether it can be sent from the source to destination here
	// Save to DB or send it straight? Idk
	// Use e.g. google pubsub if cloud and maybe kafka locally

	// FIXME - add more types :)
	sourcedatamap := map[string]string{}
	for key, value := range data {
		switch v := value.(type) {
		case string:
			sourcedatamap[key] = value.(string)
		default:
			log.Printf("unexpected type %T", v)
		}
	}

	log.Println(data)
	log.Println(requiredFields)
	log.Println(translatormap)
	log.Println(sourcedatamap)

	outputmap := map[string]string{}
	for _, translator := range translatormap {
		if translator.Src.Type == "static" {
			log.Printf("%s = %s", translator.Dst.Name, translator.Src.Value)
			outputmap[translator.Dst.Name] = translator.Src.Value
		} else {
			log.Printf("%s = %s", translator.Dst.Name, translator.Src.Name)
			outputmap[translator.Dst.Name] = sourcedatamap[translator.Src.Name]
		}
	}

	configmap := map[string]string{}
	for _, config := range schedule.AppInfo.DestinationApp.Config {
		configmap[config.Key] = config.Value
	}

	// FIXME - add function to run
	// FIXME - add reference somehow
	// FIXME - add apikey somehow
	// Just package and run really?

	// FIXME - generate from sourceapp
	outputmap["function"] = "create_alert"
	cmdArgs := []string{
		fmt.Sprintf("%s/%s/app.py", baseAppPath, foldername),
	}

	for key, value := range outputmap {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", key, value))
	}

	// COnfig map!
	for key, value := range configmap {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", key, value))
	}
	outputmap["referenceid"] = workflowId

	baseProcess := "python3"
	log.Printf("Executing: %s %s", baseProcess, strings.Join(cmdArgs, " "))
	execSubprocess(baseProcess, cmdArgs)

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

// Index = Username
func setSchedule(ctx context.Context, schedule ScheduleOld) error {
	key1 := datastore.NameKey("schedules", strings.ToLower(schedule.Id), nil)

	// New struct, to not add body, author etc
	if _, err := dbclient.Put(ctx, key1, &schedule); err != nil {
		log.Printf("Error adding schedule: %s", err)
		return err
	}

	return nil
}

//dst: {name: "title", required: "true", type: "string"}
//
//"title": "symptomDescription",
//"description": "detailedDescription",
//"type": "ticketType",
//"sourceRef": "ticketId"
//"name": "secureworks",
//"id": "e07910a06a086c83ba41827aa00b26ed",
//"description": "I AM SECUREWORKS DESC",
//"action": "Get Tickets",
//"config": {}
//"name": "thehive",
//			"id": "e07910a06a086c83ba41827aa00b26ef",
//			"description": "I AM thehive DESC",
//			"action": "Add ticket",
//			"config": [{
//				"key": "http://localhost:9000",
//				"value": "kZJmmn05j8wndOGDGvKg/D9eKub1itwO"
//			}]

func getAllScheduleApps(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	var err error
	var limit = 50

	// FIXME - add org search and public / private
	key, ok := request.URL.Query()["limit"]
	if ok {
		limit, err = strconv.Atoi(key[0])
		if err != nil {
			limit = 50
		}
	}

	// Max datastore limit
	if limit > 1000 {
		limit = 1000
	}

	// Get URLs from a database index (mapped by orborus)
	ctx := context.Background()
	q := datastore.NewQuery("appschedules").Limit(limit)
	var allappschedules ScheduleApps

	ret, err := dbclient.GetAll(ctx, q, &allappschedules.Apps)
	_ = ret
	if err != nil {
		log.Printf("Failed getting all apps: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting apps"}`)))
		return
	}

	newjson, err := json.Marshal(allappschedules)
	if err != nil {
		log.Printf("Failed unmarshal: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking"}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(newjson)
}

func setScheduleApp(ctx context.Context, app ApiYaml, id string) error {
	// id = md5(appname:appversion)
	key1 := datastore.NameKey("appschedules", id, nil)

	// New struct, to not add body, author etc
	if _, err := dbclient.Put(ctx, key1, &app); err != nil {
		log.Printf("Error adding schedule app: %s", err)
		return err
	}

	return nil
}

func findValidScheduleAppFolders(rootAppFolder string) ([]string, error) {
	rootFiles, err := ioutil.ReadDir(rootAppFolder)
	if err != nil {
		return []string{}, err
	}

	invalidRootFiles := []string{}
	invalidRootFolders := []string{}
	invalidAppFolders := []string{}
	validAppFolders := []string{}

	// This is dumb
	allowedLanguages := []string{"py", "go"}

	for _, rootfile := range rootFiles {
		if !rootfile.IsDir() {
			invalidRootFiles = append(invalidRootFiles, rootfile.Name())
			continue
		}

		appFolderLocation := fmt.Sprintf("%s/%s", rootAppFolder, rootfile.Name())
		appFiles, err := ioutil.ReadDir(appFolderLocation)
		if err != nil {
			// Invalid app folder (deleted within a few MS lol)
			log.Printf("%s", err)
			invalidRootFolders = append(invalidRootFolders, rootfile.Name())
			continue
		}

		yamlFileDone := false
		appFileExists := false
		for _, appfile := range appFiles {
			if appfile.Name() == "api.yaml" {
				err := validateAppYaml(
					fmt.Sprintf("%s/%s", appFolderLocation, appfile.Name()),
				)

				if err != nil {
					log.Printf("Error in %s: %s", fmt.Sprintf("%s/%s", rootfile.Name(), appfile.Name()), err)
					break
				}

				log.Printf("YAML FOR %s: %s IS VALID!!", rootfile.Name(), appfile.Name())
				yamlFileDone = true
			}

			for _, language := range allowedLanguages {
				if appfile.Name() == fmt.Sprintf("app.%s", language) {
					log.Printf("Appfile found for %s", rootfile.Name())
					appFileExists = true
					break
				}
			}
		}

		if !yamlFileDone || !appFileExists {
			invalidAppFolders = append(invalidAppFolders, rootfile.Name())
		} else {
			validAppFolders = append(validAppFolders, rootfile.Name())
		}
	}

	log.Printf("Invalid rootfiles: %s", strings.Join(invalidRootFiles, ", "))
	log.Printf("Invalid rootfolders: %s", strings.Join(invalidRootFolders, ", "))
	log.Printf("Invalid appfolders: %s", strings.Join(invalidAppFolders, ", "))
	log.Printf("\n=== VALID appfolders ===\n* %s", strings.Join(validAppFolders, "\n"))

	return validAppFolders, err
}

func validateInputOutputYaml(appType string, apiYaml ApiYaml) error {
	if appType == "input" {
		for index, input := range apiYaml.Input {
			if input.Name == "" {
				return errors.New(fmt.Sprintf("YAML field name doesn't exist in index %d of Input", index))
			}
			if input.Description == "" {
				return errors.New(fmt.Sprintf("YAML field description doesn't exist in index %d of Input", index))
			}

			for paramindex, param := range input.InputParameters {
				if param.Name == "" {
					return errors.New(fmt.Sprintf("YAML field name doesn't exist in Input %s with index %d", input.Name, paramindex))
				}

				if param.Description == "" {
					return errors.New(fmt.Sprintf("YAML field description doesn't exist in Input %s with index %d", input.Name, index))
				}

				if param.Schema.Type == "" {
					return errors.New(fmt.Sprintf("YAML field schema.type doesn't exist in Input %s with index %d", input.Name, index))
				}
			}
		}
	}

	return nil
}

func validateAppYaml(fileLocation string) error {
	/*
		Requires:
		name, description, app_version, contact_info (name), types
	*/

	apiYaml, err := loadYaml(fileLocation)
	if err != nil {
		return err
	}

	// Validate fields
	if apiYaml.Name == "" {
		return errors.New("YAML field name doesn't exist")
	}
	if apiYaml.Description == "" {
		return errors.New("YAML field description doesn't exist")
	}

	if apiYaml.AppVersion == "" {
		return errors.New("YAML field app_version doesn't exist")
	}

	if apiYaml.ContactInfo.Name == "" {
		return errors.New("YAML field contact_info.name doesn't exist")
	}

	if len(apiYaml.Types) == 0 {
		return errors.New("YAML field types doesn't exist")
	}

	// Validate types (input/ouput)
	validTypes := []string{"input", "output"}
	for _, appType := range apiYaml.Types {
		// Validate in here lul
		for _, validType := range validTypes {
			if appType == validType {
				err = validateInputOutputYaml(appType, apiYaml)
				if err != nil {
					return err
				}
				break
			}
		}
	}

	return nil
}

func getHook(ctx context.Context, hookId string) (*Hook, error) {
	key := datastore.NameKey("hooks", strings.ToLower(hookId), nil)
	hook := &Hook{}
	if err := dbclient.Get(ctx, key, hook); err != nil {
		return &Hook{}, err
	}

	return hook, nil
}

func setHook(ctx context.Context, hook Hook) error {
	key1 := datastore.NameKey("hooks", strings.ToLower(hook.Id), nil)

	// New struct, to not add body, author etc
	if _, err := dbclient.Put(ctx, key1, &hook); err != nil {
		log.Printf("Error adding hook: %s", err)
		return err
	}

	return nil
}

func handleGetallHooks(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("[WARNING] Api authentication failed in set new workflowhandler: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	ctx := context.Background()
	// With user, do a search for workflows with user or user's org attached
	q := datastore.NewQuery("hooks").Filter("owner =", user.Username)
	var allhooks []Hook
	_, err = dbclient.GetAll(ctx, q, &allhooks)
	if err != nil {
		log.Printf("Failed getting hooks for user %s: %s", user.Username, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if len(allhooks) == 0 {
		resp.WriteHeader(200)
		resp.Write([]byte("[]"))
		return
	}

	newjson, err := json.Marshal(allhooks)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking"}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(newjson)
}

//func deployWebhookCloudrun(ctx context.Context) {
//	service, err := cloudrun.NewService(ctx)
//	_ = err
//
//	projectsLocationsService := cloudrun.NewProjectsLocationsService(service)
//	log.Printf("%#v", projectsLocationsService)
//	projectsLocationsGetCall := projectsLocationsService.Get("webhook")
//	log.Printf("%#v", projectsLocationsGetCall)
//
//	location, err := projectsLocationsGetCall.Do()
//	log.Printf("%#v, err: %s", location, err)
//
//	//func NewProjectsLocationsService(s *Service) *ProjectsLocationsService {
//	//func (r *ProjectsLocationsService) Get(name string) *ProjectsLocationsGetCall {
//	//func (c *ProjectsLocationsGetCall) Do(opts ...googleapi.CallOption) (*Location, error) {
//}

// Finds available ports
func findAvailablePorts(startRange int64, endRange int64) string {
	for i := startRange; i < endRange; i++ {
		s := strconv.FormatInt(i, 10)
		l, err := net.Listen("tcp", ":"+s)

		if err == nil {
			l.Close()
			return s
		}
	}

	return ""
}

func handleSendalert(resp http.ResponseWriter, request *http.Request) {
	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("[WARNING] Api authentication failed in sendalert: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Role != "mail" && user.Role != "admin" {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "You don't have access to send mail"}`))
		return
	}

	// ReferenceExecution and below are for execution continuations when user inputs arrive
	type mailcheck struct {
		Targets            []string `json:"targets"`
		Body               string   `json:"body"`
		Subject            string   `json:"subject"`
		Type               string   `json:"type"`
		SenderCompany      string   `json:"sender_company"`
		ReferenceExecution string   `json:"reference_execution"`
		WorkflowId         string   `json:"workflow_id"`
		ExecutionType      string   `json:"execution_type"`
		Start              string   `json:"start"`
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Body data error on mail: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	var mailbody mailcheck
	err = json.Unmarshal(body, &mailbody)
	if err != nil {
		log.Printf("Unmarshal error on mail: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	ctx := context.Background()
	confirmMessage := `
You have a new alert from shuffler.io!

%s

Please contact us at shuffler.io or frikky@shuffler.io if there is an issue with this message.`

	parsedBody := fmt.Sprintf(confirmMessage, mailbody.Body)

	// FIXME - Make a continuation email here - might need more info from worker
	// making the request, e.g. what the next start-node is and execution_id for
	// how to make the links
	if mailbody.Type == "User input" {
		authkey := uuid.NewV4().String()

		log.Printf("Should handle differentiator for user input in email!")
		log.Printf("%#v", mailbody)

		url := "https://shuffler.io"
		//url := "http://localhost:5001"
		continueUrl := fmt.Sprintf("%s/api/v1/workflows/%s/execute?authorization=%s&start=%s&reference_execution=%s&answer=true", url, mailbody.WorkflowId, authkey, mailbody.Start, mailbody.ReferenceExecution)
		stopUrl := fmt.Sprintf("%s/api/v1/workflows/%s/execute?authorization=%s&start=%s&reference_execution=%s&answer=false", url, mailbody.WorkflowId, authkey, mailbody.Start, mailbody.ReferenceExecution)

		//item := &memcache.Item{
		//	Key:        authkey,
		//	Value:      []byte(fmt.Sprintf(`{"role": "workflow_%s"}`, mailbody.WorkflowId)),
		//	Expiration: time.Minute * 1200,
		//}

		//if err := memcache.Add(ctx, item); err == memcache.ErrNotStored {
		//	if err := memcache.Set(ctx, item); err != nil {
		//		log.Printf("Error setting new user item: %v", err)
		//	}
		//} else if err != nil {
		//	log.Printf("error adding item: %v", err)
		//} else {
		//	log.Printf("Set cache for %s", item.Key)
		//}

		parsedBody = fmt.Sprintf(`
Action required!
			
%s

If this is TRUE click this: %s

IF THIS IS FALSE, click this: %s

Please contact us at shuffler.io or frikky@shuffler.io if there is an issue with this message.
`, mailbody.Body, continueUrl, stopUrl)

	}

	msg := &mail.Message{
		Sender:  "Shuffle <frikky@shuffler.io>",
		To:      mailbody.Targets,
		Subject: fmt.Sprintf("Shuffle - %s - %s", mailbody.Type, mailbody.Subject),
		Body:    parsedBody,
	}

	log.Println(msg.Body)
	if err := mail.Send(ctx, msg); err != nil {
		log.Printf("Couldn't send email: %v", err)
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

func setBadMemcache(ctx context.Context, path string) {
	// Add to cache if it doesn't exist
	//item := &memcache.Item{
	//	Key:        path,
	//	Value:      []byte(`{"success": false}`),
	//	Expiration: time.Minute * 60,
	//}

	//if err := memcache.Add(ctx, item); err == memcache.ErrNotStored {
	//	if err := memcache.Set(ctx, item); err != nil {
	//		log.Printf("Error setting item: %v", err)
	//	}
	//} else if err != nil {
	//	log.Printf("error adding item: %v", err)
	//} else {
	//	log.Printf("Set cache for %s", item.Key)
	//}
}

type Result struct {
	Success bool     `json:"success"`
	Reason  string   `json:"reason"`
	List    []string `json:"list"`
}

var docs_list = Result{List: []string{}}

func getDocList(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	ctx := context.Background()
	//if item, err := memcache.Get(ctx, "docs_list"); err == memcache.ErrCacheMiss {
	//	// Not in cache
	//} else if err != nil {
	//	// Error with cache
	//	log.Printf("Error getting item: %v", err)
	//} else {
	//	resp.WriteHeader(200)
	//	resp.Write([]byte(item.Value))
	//	return
	//}

	if len(docs_list.List) > 0 {
		b, err := json.Marshal(docs_list)
		if err != nil {
			log.Printf("Failed marshaling result: %s", err)
			//http.Error(resp, err.Error(), 500)
		} else {
			resp.WriteHeader(200)
			resp.Write(b)
			return
		}
	}

	client := github.NewClient(nil)
	_, item1, _, err := client.Repositories.GetContents(ctx, "frikky", "shuffle-docs", "docs", nil)
	if err != nil {
		log.Printf("Github error: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Error listing directory: %s"`, err)))
		return
	}

	if len(item1) == 0 {
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "No docs available."}`)))
		return
	}

	names := []string{}
	for _, item := range item1 {
		if !strings.HasSuffix(*item.Name, "md") {
			continue
		}

		names = append(names, (*item.Name)[0:len(*item.Name)-3])
	}

	log.Println(names)

	var result Result
	result.Success = true
	result.Reason = "Success"
	result.List = names
	docs_list = result

	b, err := json.Marshal(result)
	if err != nil {
		http.Error(resp, err.Error(), 500)
		return
	}

	//item := &memcache.Item{
	//	Key:        "docs_list",
	//	Value:      b,
	//	Expiration: time.Minute * 60,
	//}

	//if err := memcache.Add(ctx, item); err == memcache.ErrNotStored {
	//	if err := memcache.Set(ctx, item); err != nil {
	//		log.Printf("Error setting item: %v", err)
	//	}
	//} else if err != nil {
	//	log.Printf("error adding item: %v", err)
	//} else {
	//	log.Printf("Set cache for %s", item.Key)
	//}

	resp.WriteHeader(200)
	resp.Write(b)
}

// r.HandleFunc("/api/v1/docs/{key}", getDocs).Methods("GET", "OPTIONS")
var alldocs = map[string][]byte{}

func getDocs(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	location := strings.Split(request.URL.String(), "/")
	if len(location) != 5 {
		resp.WriteHeader(404)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Bad path. Use e.g. /api/v1/docs/workflows.md"}`)))
		return
	}

	//ctx := context.Background()
	docPath := fmt.Sprintf("https://raw.githubusercontent.com/shaffuru/shuffle-docs/master/docs/%s.md", location[4])
	//location[4]
	//var, ok := alldocs["asd"]
	key, ok := alldocs[fmt.Sprintf("%s", location[4])]
	// Custom cache for github issues lol
	if ok {
		resp.WriteHeader(200)
		resp.Write(key)
		return
	}

	client := &http.Client{}
	req, err := http.NewRequest(
		"GET",
		docPath,
		nil,
	)

	if err != nil {
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Bad path. Use e.g. /api/v1/docs/workflows.md"}`)))
		resp.WriteHeader(404)
		//setBadMemcache(ctx, docPath)
		return
	}

	newresp, err := client.Do(req)
	if err != nil {
		resp.WriteHeader(404)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Bad path. Use e.g. /api/v1/docs/workflows.md"}`)))
		//setBadMemcache(ctx, docPath)
		return
	}

	body, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Can't parse data"}`)))
		//setBadMemcache(ctx, docPath)
		return
	}

	type Result struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
	}

	var result Result
	result.Success = true

	//applog.Infof(ctx, string(body))
	//applog.Infof(ctx, "Url: %s", docPath)
	//applog.Infof(ctx, "Status: %d", newresp.StatusCode)
	//applog.Infof(ctx, "GOT BODY OF LENGTH %d", len(string(body)))

	result.Reason = string(body)
	b, err := json.Marshal(result)
	if err != nil {
		http.Error(resp, err.Error(), 500)
		//setBadMemcache(ctx, docPath)
		return
	}

	alldocs[location[4]] = b

	// Add to cache if it doesn't exist
	//item := &memcache.Item{
	//	Key:        docPath,
	//	Value:      b,
	//	Expiration: time.Minute * 60,
	//}

	//if err := memcache.Add(ctx, item); err == memcache.ErrNotStored {
	//	if err := memcache.Set(ctx, item); err != nil {
	//		log.Printf("Error setting item: %v", err)
	//	}
	//} else if err != nil {
	//	log.Printf("error adding item: %v", err)
	//} else {
	//	log.Printf("Set cache for %s", item.Key)
	//}

	resp.WriteHeader(200)
	resp.Write(b)
}

func handleGetSpecificStats(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	_, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in getting specific workflow: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var statsId string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		statsId = location[4]
	}

	ctx := context.Background()
	statisticsId := "global_statistics"
	nameKey := statsId
	key := datastore.NameKey(statisticsId, nameKey, nil)
	statisticsItem := StatisticsItem{}
	if err := dbclient.Get(ctx, key, &statisticsItem); err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	b, err := json.Marshal(statisticsItem)
	if err != nil {
		log.Printf("Failed to marshal data: %s", err)
		resp.WriteHeader(401)
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(b))
}

func getOpenapi(resp http.ResponseWriter, request *http.Request) {
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

	location := strings.Split(request.URL.String(), "/")
	var id string
	if location[1] == "api" {
		if len(location) <= 4 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		id = location[4]
	}

	if len(id) != 32 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - FIX AUTH WITH APP
	ctx := context.Background()
	//_, err = shuffle.GetApp(ctx, id)
	//if err == nil {
	//	log.Println("You're supposed to be able to continue now.")
	//}

	parsedApi, err := getOpenApiDatastore(ctx, id)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	log.Printf("[INFO] API LENGTH GET FOR OPENAPI %s: %d, ID: %s", id, len(parsedApi.Body), id)

	parsedApi.Success = true
	data, err := json.Marshal(parsedApi)
	if err != nil {
		resp.WriteHeader(422)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed marshalling parsed swagger: %s"}`, err)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(data)
}

func echoOpenapiData(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// Just here to verify that the user is logged in
	_, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in validate swagger: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Failed authentication"}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Bodyreader err: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Failed reading body"}`))
		return
	}

	newbody := string(body)
	newbody = strings.TrimSpace(newbody)
	if strings.HasPrefix(newbody, "\"") {
		newbody = newbody[1:len(newbody)]
	}

	if strings.HasSuffix(newbody, "\"") {
		newbody = newbody[0 : len(newbody)-1]
	}

	req, err := http.NewRequest("GET", newbody, nil)
	if err != nil {
		log.Printf("[ERROR] Requestbuilder err: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Failed building request"}`))
		return
	}

	httpClient := &http.Client{}
	newresp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] Grabbing error: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed making remote request to get the data"}`)))
		return
	}
	defer newresp.Body.Close()

	urlbody, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		log.Printf("[ERROR] URLbody error: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Can't get data from selected uri"}`)))
		return
	}

	if newresp.StatusCode >= 400 {
		resp.WriteHeader(201)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, urlbody)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(urlbody)
}

func handleSwaggerValidation(body []byte) (ParsedOpenApi, error) {
	type versionCheck struct {
		Swagger        string `datastore:"swagger" json:"swagger" yaml:"swagger"`
		SwaggerVersion string `datastore:"swaggerVersion" json:"swaggerVersion" yaml:"swaggerVersion"`
		OpenAPI        string `datastore:"openapi" json:"openapi" yaml:"openapi"`
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
	var version versionCheck

	parsed := ParsedOpenApi{}
	swaggerdata := []byte{}
	idstring := ""

	isJson := false
	err := json.Unmarshal(body, &version)
	if err != nil {
		//log.Printf("Json err: %s", err)
		err = yaml.Unmarshal(body, &version)
		if err != nil {
			log.Printf("Yaml error (1): %s", err)
		} else {
			//log.Printf("Successfully parsed YAML!")
		}
	} else {
		isJson = true
		log.Printf("Successfully parsed JSON!")
	}

	if len(version.SwaggerVersion) > 0 && len(version.Swagger) == 0 {
		version.Swagger = version.SwaggerVersion
	}

	if strings.HasPrefix(version.Swagger, "3.") || strings.HasPrefix(version.OpenAPI, "3.") {
		//log.Println("Handling v3 API")
		swaggerLoader := openapi3.NewSwaggerLoader()
		swaggerLoader.IsExternalRefsAllowed = true
		swaggerv3, err := swaggerLoader.LoadSwaggerFromData(body)
		if err != nil {
			log.Printf("Failed parsing OpenAPI: %s", err)
			return ParsedOpenApi{}, err
		}

		swaggerdata, err = json.Marshal(swaggerv3)
		if err != nil {
			log.Printf("Failed unmarshaling v3 data: %s", err)
			return ParsedOpenApi{}, err
		}

		hasher := md5.New()
		hasher.Write(swaggerdata)
		idstring = hex.EncodeToString(hasher.Sum(nil))

	} else { //strings.HasPrefix(version.Swagger, "2.") || strings.HasPrefix(version.OpenAPI, "2.") {
		// Convert
		//log.Println("Handling v2 API")
		var swagger openapi2.Swagger
		//log.Println(string(body))
		err = json.Unmarshal(body, &swagger)
		if err != nil {
			//log.Printf("Json error? %s", err)
			err = yaml.Unmarshal(body, &swagger)
			if err != nil {
				log.Printf("Yaml error (2): %s", err)
				return ParsedOpenApi{}, err
			} else {
				//log.Printf("Valid yaml!")
			}

		}

		swaggerv3, err := openapi2conv.ToV3Swagger(&swagger)
		if err != nil {
			log.Printf("Failed converting from openapi2 to 3: %s", err)
			return ParsedOpenApi{}, err
		}

		swaggerdata, err = json.Marshal(swaggerv3)
		if err != nil {
			log.Printf("Failed unmarshaling v3 data: %s", err)
			return ParsedOpenApi{}, err
		}

		hasher := md5.New()
		hasher.Write(swaggerdata)
		idstring = hex.EncodeToString(hasher.Sum(nil))
	}

	if len(swaggerdata) > 0 {
		body = swaggerdata
	}

	// Overwrite with new json data
	_ = isJson
	body = swaggerdata

	// Parsing it to swagger 3
	parsed = ParsedOpenApi{
		ID:      idstring,
		Body:    string(body),
		Success: true,
	}

	return parsed, err
}

// FIXME: Migrate this to use handleSwaggerValidation()
func validateSwagger(resp http.ResponseWriter, request *http.Request) {
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

	type versionCheck struct {
		Swagger        string `datastore:"swagger" json:"swagger" yaml:"swagger"`
		SwaggerVersion string `datastore:"swaggerVersion" json:"swaggerVersion" yaml:"swaggerVersion"`
		OpenAPI        string `datastore:"openapi" json:"openapi" yaml:"openapi"`
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
	var version versionCheck

	log.Printf("API length SET: %d", len(string(body)))

	isJson := false
	err = json.Unmarshal(body, &version)
	if err != nil {
		log.Printf("Json err: %s", err)
		err = yaml.Unmarshal(body, &version)
		if err != nil {
			log.Printf("Yaml error (3): %s", err)
			resp.WriteHeader(422)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed reading openapi to json and yaml. Is version defined?: %s"}`, err)))
			return
		} else {
			log.Printf("[INFO] Successfully parsed YAML (3)!")
		}
	} else {
		isJson = true
		log.Printf("[INFO] Successfully parsed JSON!")
	}

	if len(version.SwaggerVersion) > 0 && len(version.Swagger) == 0 {
		version.Swagger = version.SwaggerVersion
	}
	log.Printf("[INFO] Version: %#v", version)
	log.Printf("[INFO] OpenAPI: %s", version.OpenAPI)

	if strings.HasPrefix(version.Swagger, "3.") || strings.HasPrefix(version.OpenAPI, "3.") {
		log.Println("[INFO] Handling v3 API")
		swaggerLoader := openapi3.NewSwaggerLoader()
		swaggerLoader.IsExternalRefsAllowed = true
		swagger, err := swaggerLoader.LoadSwaggerFromData(body)
		if err != nil {
			log.Printf("[WARNING] Failed to convert v3 API: %s", err)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
			return
		}

		hasher := md5.New()
		hasher.Write(body)
		idstring := hex.EncodeToString(hasher.Sum(nil))

		log.Printf("Swagger v3 validation success with ID %s and %d paths!", idstring, len(swagger.Paths))

		if !isJson {
			log.Printf("[INFO] NEED TO TRANSFORM FROM YAML TO JSON for %s", idstring)
		}

		swaggerdata, err := json.Marshal(swagger)
		if err != nil {
			log.Printf("Failed unmarshaling v3 data: %s", err)
			resp.WriteHeader(422)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed marshalling swaggerv3 data: %s"}`, err)))
			return
		}
		parsed := ParsedOpenApi{
			ID:   idstring,
			Body: string(swaggerdata),
		}

		ctx := context.Background()
		err = setOpenApiDatastore(ctx, idstring, parsed)
		if err != nil {
			log.Printf("Failed uploading openapi to datastore: %s", err)
			resp.WriteHeader(422)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed reading openapi2: %s"}`, err)))
			return
		}

		log.Printf("[INFO] Successfully set OpenAPI with ID %s", idstring)
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "id": "%s"}`, idstring)))
		return
	} else { //strings.HasPrefix(version.Swagger, "2.") || strings.HasPrefix(version.OpenAPI, "2.") {
		// Convert
		log.Println("Handling v2 API")
		var swagger openapi2.Swagger
		//log.Println(string(body))
		err = json.Unmarshal(body, &swagger)
		if err != nil {
			log.Printf("Json error for v2 - trying yaml: %s", err)
			err = yaml.Unmarshal([]byte(body), &swagger)
			if err != nil {
				log.Printf("Yaml error (4): %s", err)

				resp.WriteHeader(422)
				resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed reading openapi2: %s"}`, err)))
				return
			} else {
				log.Printf("Found valid yaml!")
			}

		}

		swaggerv3, err := openapi2conv.ToV3Swagger(&swagger)
		if err != nil {
			log.Printf("Failed converting from openapi2 to 3: %s", err)
			resp.WriteHeader(422)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed converting from openapi2 to openapi3: %s"}`, err)))
			return
		}

		swaggerdata, err := json.Marshal(swaggerv3)
		if err != nil {
			log.Printf("Failed unmarshaling v3 from v2 data: %s", err)
			resp.WriteHeader(422)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed marshalling swaggerv3 data: %s"}`, err)))
			return
		}

		hasher := md5.New()
		hasher.Write(swaggerdata)
		idstring := hex.EncodeToString(hasher.Sum(nil))
		if !isJson {
			log.Printf("FIXME: NEED TO TRANSFORM FROM YAML TO JSON for %s?", idstring)
		}
		log.Printf("Swagger v2 -> v3 validation success with ID %s!", idstring)

		parsed := ParsedOpenApi{
			ID:   idstring,
			Body: string(swaggerdata),
		}

		ctx := context.Background()
		err = setOpenApiDatastore(ctx, idstring, parsed)
		if err != nil {
			log.Printf("Failed uploading openapi2 to datastore: %s", err)
			resp.WriteHeader(422)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed reading openapi2: %s"}`, err)))
			return
		}

		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "id": "%s"}`, idstring)))
		return
	}
	/*
		else {
			log.Printf("Swagger / OpenAPI version %s is not supported or there is an error.", version.Swagger)
			resp.WriteHeader(422)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Swagger version %s is not currently supported"}`, version.Swagger)))
			return
		}
	*/

	// save the openapi ID
	resp.WriteHeader(422)
	resp.Write([]byte(`{"success": false}`))
}

// Creates an app from the app builder
func verifySwagger(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	log.Printf("[INFO] SETTING APP TO LIVE!!!")
	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in verify swagger: %s", err)
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

	type Test struct {
		Editing bool   `datastore:"editing"`
		Id      string `datastore:"id"`
		Image   string `datastore:"image"`
	}

	var test Test
	err = json.Unmarshal(body, &test)
	if err != nil {
		log.Printf("Failed unmarshalling test: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Get an identifier
	hasher := md5.New()
	hasher.Write(body)
	newmd5 := hex.EncodeToString(hasher.Sum(nil))
	if test.Editing {
		// Quick verification test
		ctx := context.Background()
		app, err := shuffle.GetApp(ctx, test.Id, user)
		if err != nil {
			log.Printf("Error getting app when editing: %s", app.Name)
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		// FIXME: Check whether it's in use.
		if user.Id != app.Owner && user.Role != "admin" {
			log.Printf("[WARNING] Wrong user (%s) for app %s when verifying swagger", user.Username, app.Name)
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		log.Printf("[INFO] EDITING APP WITH ID %s", app.ID)
		newmd5 = app.ID
	}

	// Generate new app integration (bump version)
	// Test = client side with fetch?

	ctx := context.Background()
	swaggerLoader := openapi3.NewSwaggerLoader()
	swaggerLoader.IsExternalRefsAllowed = true
	swagger, err := swaggerLoader.LoadSwaggerFromData(body)
	if err != nil {
		log.Printf("[ERROR] Swagger validation error: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Failed verifying openapi"}`))
		return
	}

	if swagger.Info == nil {
		log.Printf("[ERORR] Info is nil?: %#v", swagger)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Info not parsed"}`))
		return
	}

	if strings.Contains(swagger.Info.Title, " ") {
		swagger.Info.Title = strings.Replace(swagger.Info.Title, " ", "_", -1)
	}

	basePath, err := shuffle.BuildStructure(swagger, newmd5)
	if err != nil {
		log.Printf("Failed to build base structure: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Failed building baseline structure"}`))
		return
	}

	//log.Printf("Should generate yaml")
	swagger, api, pythonfunctions, err := shuffle.GenerateYaml(swagger, newmd5)
	if err != nil {
		log.Printf("Failed building and generating yaml: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Failed building and parsing yaml"}`))
		return
	}

	// FIXME: CHECK IF SAME NAME AS NORMAL APP
	// Can't overwrite existing normal app
	workflowApps, err := shuffle.GetPrioritizedApps(ctx, user)
	if err != nil {
		log.Printf("Failed getting all workflow apps from database to verify: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Failed to verify existence"}`))
		return
	}

	// Same name only?
	lowerName := strings.ToLower(swagger.Info.Title)
	for _, app := range workflowApps {
		if app.Downloaded && !app.Generated && strings.ToLower(app.Name) == lowerName {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Normal app with name %s already exists. Delete it first."}`, swagger.Info.Title)))
			return
		}
	}

	api.Owner = user.Id

	err = shuffle.DumpApi(basePath, api)
	if err != nil {
		log.Printf("Failed dumping yaml: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Failed dumping yaml"}`))
		return
	}

	identifier := fmt.Sprintf("%s-%s", swagger.Info.Title, newmd5)
	classname := strings.Replace(identifier, " ", "", -1)
	classname = strings.Replace(classname, "-", "", -1)
	parsedCode, err := shuffle.DumpPython(basePath, classname, swagger.Info.Version, pythonfunctions)
	if err != nil {
		log.Printf("Failed dumping python: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Failed dumping appcode"}`))
		return
	}

	identifier = strings.Replace(identifier, " ", "-", -1)
	identifier = strings.Replace(identifier, "_", "-", -1)
	log.Printf("[INFO] Successfully parsed %s. Proceeding to docker container", identifier)

	// Now that the baseline is setup, we need to make it into a cloud function
	// 1. Upload the API to datastore for use
	// 2. Get code from baseline/app_base.py & baseline/static_baseline.py
	// 3. Stitch code together from these two + our new app
	// 4. Zip the folder to cloud storage
	// 5. Upload as cloud function

	// 1. Upload the API to datastore
	err = shuffle.DeployAppToDatastore(ctx, api)
	//func DeployAppToDatastore(ctx context.Context, workflowapp WorkflowApp, bucketName string) error {
	if err != nil {
		log.Printf("Failed adding app to db: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed adding app to db: %s"}`, err)))
		return
	}

	// 2. Get all the required code
	appbase, staticBaseline, err := shuffle.GetAppbase()
	if err != nil {
		log.Printf("Failed getting appbase: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Failed getting appbase code"}`))
		return
	}

	// Have to do some quick checks of the python code (:
	_, parsedCode = shuffle.FormatAppfile(parsedCode)

	fixedAppbase := shuffle.FixAppbase(appbase)
	runner := shuffle.GetRunnerOnprem(classname)

	// 2. Put it together
	stitched := string(staticBaseline) + strings.Join(fixedAppbase, "\n") + parsedCode + string(runner)
	//log.Println(stitched)

	// 3. Zip and stream it directly in the directory
	_, err = shuffle.StreamZipdata(ctx, identifier, stitched, "requests\nurllib3", "")
	if err != nil {
		log.Printf("[ERROR] Zipfile error: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(`{"success": false, "reason": "Failed to build zipfile"}`))
		return
	}

	log.Printf("[INFO] Successfully stitched ZIPFILE for %s", identifier)

	// 4. Upload as cloud function - this apikey is specifically for cloud functions rofl
	//environmentVariables := map[string]string{
	//	"FUNCTION_APIKEY": apikey,
	//}

	//fullLocation := fmt.Sprintf("gs://%s/%s", bucketName, applocation)
	//err = deployCloudFunctionPython(ctx, identifier, defaultLocation, fullLocation, environmentVariables)
	//if err != nil {
	//	log.Printf("Error uploading cloud function: %s", err)
	//	resp.WriteHeader(500)
	//	resp.Write([]byte(`{"success": false, "reason": "Failed to upload function"}`))
	//	return
	//}

	// 4. Build the image locally.
	// FIXME: Should be moved to a local docker registry
	dockerLocation := fmt.Sprintf("%s/Dockerfile", basePath)
	log.Printf("[INFO] Dockerfile: %s", dockerLocation)

	versionName := fmt.Sprintf("%s_%s", strings.ToLower(strings.ReplaceAll(api.Name, " ", "-")), api.AppVersion)
	dockerTags := []string{
		fmt.Sprintf("%s:%s", baseDockerName, identifier),
		fmt.Sprintf("%s:%s", baseDockerName, versionName),
	}

	err = buildImage(dockerTags, dockerLocation)
	if err != nil {
		log.Printf("[ERROR] Docker build error: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Error in Docker build: %s"}`, err)))
		return
	}

	found := false
	foundNumber := 0
	log.Printf("[INFO] Checking for api with ID %s", newmd5)
	for appCounter, app := range user.PrivateApps {
		if app.ID == api.ID {
			found = true
			foundNumber = appCounter
			break
		} else if app.Name == api.Name && app.AppVersion == api.AppVersion {
			found = true
			foundNumber = appCounter
			break
		} else if app.PrivateID == test.Id && test.Editing {
			found = true
			foundNumber = appCounter
			break
		}
	}

	// Updating the user with the new app so that it can easily be retrieved
	if !found {
		user.PrivateApps = append(user.PrivateApps, api)
	} else {
		user.PrivateApps[foundNumber] = api
	}

	err = shuffle.SetUser(ctx, &user)
	if err != nil {
		log.Printf("[ERROR] Failed adding verification for user %s: %s", user.Username, err)
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "Failed updating user"}`)))
		return
	}

	//log.Printf("DO I REACH HERE WHEN SAVING?")
	parsed := ParsedOpenApi{
		ID:   newmd5,
		Body: string(body),
	}

	log.Printf("[INFO] API LENGTH FOR %s: %d, ID: %s", api.Name, len(parsed.Body), newmd5)
	// FIXME: Might cause versioning issues if we re-use the same!!
	// FIXME: Need a way to track different versions of the same app properly.
	// Hint: Save API.id somewhere, and use newmd5 to save latest version
	err = setOpenApiDatastore(ctx, newmd5, parsed)
	if err != nil {
		log.Printf("[ERROR] Failed saving to datastore: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "%"}`, err)))
	}

	// Backup every single one
	setOpenApiDatastore(ctx, api.ID, parsed)

	/*
		err = increaseStatisticsField(ctx, "total_apps_created", newmd5, 1, user.ActiveOrg.Id)
		if err != nil {
			log.Printf("Failed to increase success execution stats: %s", err)
		}

		err = increaseStatisticsField(ctx, "openapi_apps_created", newmd5, 1, user.ActiveOrg.Id)
		if err != nil {
			log.Printf("Failed to increase success execution stats: %s", err)
		}
	*/

	cacheKey := fmt.Sprintf("workflowapps-sorted-100")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-500")
	shuffle.DeleteCache(ctx, cacheKey)
	shuffle.DeleteCache(ctx, fmt.Sprintf("apps_%s", user.Id))

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true, "id": "%s"}`, api.ID)))
}

func healthCheckHandler(resp http.ResponseWriter, request *http.Request) {
	fmt.Fprint(resp, "OK")
}

// Creates osfs from folderpath with a basepath as directory base
func createFs(basepath, pathname string) (billy.Filesystem, error) {
	log.Printf("[INFO] MemFS base: %s, pathname: %s", basepath, pathname)

	fs := memfs.New()
	err := filepath.Walk(pathname,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if strings.Contains(path, ".git") {
				return nil
			}

			// Fix the inner path here
			newpath := strings.ReplaceAll(path, pathname, "")
			fullpath := fmt.Sprintf("%s%s", basepath, newpath)
			switch mode := info.Mode(); {
			case mode.IsDir():
				err = fs.MkdirAll(fullpath, 0644)
				if err != nil {
					log.Printf("Failed making folder: %s", err)
				}
			case mode.IsRegular():
				srcData, err := ioutil.ReadFile(path)
				if err != nil {
					log.Printf("Src error: %s", err)
					return err
				}

				//if strings.Contains(path, "yaml") {
				//	log.Printf("PATH: %s -> %s", path, fullpath)
				//	//log.Printf("DATA: %s", string(srcData))
				//}

				dst, err := fs.Create(fullpath)
				if err != nil {
					log.Printf("Dst error: %s", err)
					return err
				}

				_, err = dst.Write(srcData)
				if err != nil {
					log.Printf("Dst write error: %s", err)
					return err
				}
			}

			return nil
		})

	return fs, err
}

// Hotloads new apps from a folder
func handleAppHotload(ctx context.Context, location string, forceUpdate bool) error {

	basepath := "base"
	fs, err := createFs(basepath, location)
	if err != nil {
		log.Printf("Failed memfs creation - probably bad path: %s", err)
		return errors.New(fmt.Sprintf("Failed to find directory %s", location))
	} else {
		log.Printf("[INFO] Memfs creation from %s done", location)
	}

	dir, err := fs.ReadDir("")
	if err != nil {
		log.Printf("[WARNING] Failed reading folder: %s", err)
		return err
	}

	//log.Printf("Reading app folder: %#v", dir)
	_, _, err = iterateAppGithubFolders(fs, dir, "", "", forceUpdate)
	if err != nil {
		log.Printf("[WARNING] Githubfolders error: %s", err)
		return err
	}

	cacheKey := fmt.Sprintf("workflowapps-sorted")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-100")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-500")
	shuffle.DeleteCache(ctx, cacheKey)
	//shuffle.DeleteCache(ctx, fmt.Sprintf("apps_%s", user.Id))

	return nil
}

func handleCloudExecutionOnprem(workflowId, startNode, executionSource, executionArgument string) error {
	ctx := context.Background()
	// 1. Get the workflow
	// 2. Execute it with the data
	workflow, err := shuffle.GetWorkflow(ctx, workflowId)
	if err != nil {
		return err
	}

	// FIXME: Handle auth
	_ = workflow

	parsedArgument := executionArgument
	newExec := shuffle.ExecutionRequest{
		ExecutionSource:   executionSource,
		ExecutionArgument: parsedArgument,
	}

	var execution shuffle.ExecutionRequest
	err = json.Unmarshal([]byte(parsedArgument), &execution)
	if err == nil {
		//log.Printf("[INFO] FOUND EXEC %#v", execution)
		if len(execution.ExecutionArgument) > 0 {
			parsedArgument := strings.Replace(string(execution.ExecutionArgument), "\\\"", "\"", -1)
			log.Printf("New exec argument: %s", execution.ExecutionArgument)

			if strings.HasPrefix(parsedArgument, "{") && strings.HasSuffix(parsedArgument, "}") {
				log.Printf("\nData is most likely JSON from %s\n", newExec.ExecutionSource)
			}

			newExec.ExecutionArgument = parsedArgument
		}
	} else {
		log.Printf("Unmarshal issue: %s", err)
	}

	if len(startNode) > 0 {
		newExec.Start = startNode
	}

	b, err := json.Marshal(newExec)
	if err != nil {
		log.Printf("Failed marshal")
		return err
	}

	//log.Println(string(b))
	newRequest := &http.Request{
		URL:    &url.URL{},
		Method: "POST",
		Body:   ioutil.NopCloser(bytes.NewReader(b)),
	}

	_, _, err = handleExecution(workflowId, shuffle.Workflow{}, newRequest)
	return err
}

func handleCloudJob(job shuffle.CloudSyncJob) error {
	// May need authentication in all of these..?

	log.Printf("[INFO] Handle job with type %s and action %s", job.Type, job.Action)
	if job.Type == "outlook" {
		if job.Action == "execute" {
			// FIXME: Get the email
			ctx := context.Background()
			maildata := MailData{}
			err := json.Unmarshal([]byte(job.ThirdItem), &maildata)
			if err != nil {
				log.Printf("Maildata unmarshal error: %s", err)
				return err
			}

			hookId := job.Id
			hook, err := getTriggerAuth(ctx, hookId)
			if err != nil {
				log.Printf("[INFO] Failed getting trigger %s (callback cloud): %s", hookId, err)
				return err
			}

			redirectDomain := "localhost:5001"
			redirectUrl := fmt.Sprintf("http://%s/api/v1/triggers/outlook/register", redirectDomain)
			outlookClient, _, err := getOutlookClient(ctx, "", hook.OauthToken, redirectUrl)
			if err != nil {
				log.Printf("Oauth client failure - triggerauth: %s", err)
				return err
			}

			emails, err := getOutlookEmail(outlookClient, maildata)
			//log.Printf("EMAILS: %d", len(emails))
			//log.Printf("INSIDE GET OUTLOOK EMAIL!: %#v, %s", emails, err)

			//type FullEmail struct {
			email := FullEmail{}
			if len(emails) == 1 {
				email = emails[0]
			}

			emailBytes, err := json.Marshal(email)
			if err != nil {
				log.Printf("[INFO] Failed email marshaling: %s", err)
				return err
			}

			log.Printf("[INFO] Should handle outlook webhook for workflow %s with start node %s and data of length %d", job.PrimaryItemId, job.SecondaryItem, len(job.ThirdItem))
			err = handleCloudExecutionOnprem(job.PrimaryItemId, job.SecondaryItem, "outlook", string(emailBytes))
			if err != nil {
				log.Printf("[WARNING] Failed executing workflow from cloud outlook hook: %s", err)
			} else {
				log.Printf("[INFO] Successfully executed workflow from cloud outlook hook!")
			}
		}
	} else if job.Type == "webhook" {
		if job.Action == "execute" {
			log.Printf("[INFO] Should handle normal webhook for workflow %s with start node %s and data %s", job.PrimaryItemId, job.SecondaryItem, job.ThirdItem)
			err := handleCloudExecutionOnprem(job.PrimaryItemId, job.SecondaryItem, "webhook", job.ThirdItem)
			if err != nil {
				log.Printf("[INFO] Failed executing workflow from cloud hook: %s", err)
			} else {
				log.Printf("[INFO] Successfully executed workflow from cloud hook!")
			}
		}

	} else if job.Type == "schedule" {
		if job.Action == "execute" {
			log.Printf("Should handle schedule for workflow %s with start node %s and data %s", job.PrimaryItemId, job.SecondaryItem, job.ThirdItem)
			err := handleCloudExecutionOnprem(job.PrimaryItemId, job.SecondaryItem, "schedule", job.ThirdItem)
			if err != nil {
				log.Printf("[INFO] Failed executing workflow from cloud schedule: %s", err)
			} else {
				log.Printf("[INFO] Successfully executed workflow from cloud schedule")
			}
		}
	} else if job.Type == "email_trigger" {
		if job.Action == "execute" {
			log.Printf("Should handle email for workflow %s with start node %s and data %s", job.PrimaryItemId, job.SecondaryItem, job.ThirdItem)
			err := handleCloudExecutionOnprem(job.PrimaryItemId, job.SecondaryItem, "email_trigger", job.ThirdItem)
			if err != nil {
				log.Printf("Failed executing workflow from email trigger: %s", err)
			} else {
				log.Printf("Successfully executed workflow from cloud email trigger")
			}
		}

	} else if job.Type == "user_input" {
		if job.Action == "continue" {
			log.Printf("Should handle user_input CONTINUE for workflow %s with start node %s and execution ID %s", job.PrimaryItemId, job.SecondaryItem, job.ThirdItem)
			// FIXME: Handle authorization
			ctx := context.Background()
			workflowExecution, err := shuffle.GetWorkflowExecution(ctx, job.ThirdItem)
			if err != nil {
				return err
			}

			if job.PrimaryItemId != workflowExecution.Workflow.ID {
				return errors.New("Bad workflow ID when stopping execution.")
			}

			workflowExecution.Status = "EXECUTING"
			err = shuffle.SetWorkflowExecution(ctx, *workflowExecution, true)
			if err != nil {
				return err
			}

			fullUrl := fmt.Sprintf("%s/api/v1/workflows/%s/execute?authorization=%s&start=%s&reference_execution=%s&answer=true", syncUrl, job.PrimaryItemId, job.FourthItem, job.SecondaryItem, job.ThirdItem)
			newRequest, err := http.NewRequest(
				"GET",
				fullUrl,
				nil,
			)
			if err != nil {
				log.Printf("Failed continuing workflow in request builder: %s", err)
				return err
			}

			_, _, err = handleExecution(job.PrimaryItemId, shuffle.Workflow{}, newRequest)
			if err != nil {
				log.Printf("Failed continuing workflow from cloud user_input: %s", err)
				return err
			} else {
				log.Printf("Successfully executed workflow from cloud user_input")
			}
		} else if job.Action == "stop" {
			log.Printf("Should handle user_input STOP for workflow %s with start node %s and execution ID %s", job.PrimaryItemId, job.SecondaryItem, job.ThirdItem)
			ctx := context.Background()
			workflowExecution, err := shuffle.GetWorkflowExecution(ctx, job.ThirdItem)
			if err != nil {
				return err
			}

			if job.PrimaryItemId != workflowExecution.Workflow.ID {
				return errors.New("Bad workflow ID when stopping execution.")
			}

			/*
				if job.FourthItem != workflowExecution.Authorization {
					return errors.New("Bad authorization when stopping execution.")
				}
			*/

			newResults := []shuffle.ActionResult{}
			for _, result := range workflowExecution.Results {
				if result.Action.AppName == "User Input" && result.Result == "Waiting for user feedback based on configuration" {
					result.Status = "ABORTED"
					result.Result = "Aborted manually by user."
				}

				newResults = append(newResults, result)
			}

			workflowExecution.Results = newResults
			workflowExecution.Status = "ABORTED"
			err = shuffle.SetWorkflowExecution(ctx, *workflowExecution, true)
			if err != nil {
				return err
			}

			log.Printf("Successfully updated user input to aborted.")
		}
	} else {
		log.Printf("No handler for type %s and action %s", job.Type, job.Action)
	}

	return nil
}

// Handles jobs from remote (cloud)
func remoteOrgJobController(org shuffle.Org, body []byte) error {
	type retStruct struct {
		Success bool                   `json:"success"`
		Reason  string                 `json:"reason"`
		Jobs    []shuffle.CloudSyncJob `json:"jobs"`
	}

	responseData := retStruct{}
	err := json.Unmarshal(body, &responseData)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if !responseData.Success {
		log.Printf("[WARNING] Should stop org job controller because no success?")

		if strings.Contains(responseData.Reason, "Bad apikey") || strings.Contains(responseData.Reason, "Error getting the organization") || strings.Contains(responseData.Reason, "Organization isn't syncing") {
			log.Printf("[WARNING] Remote error; Bad apikey or org error. Stopping sync for org: %s", responseData.Reason)

			if value, exists := scheduledOrgs[org.Id]; exists {
				// Looks like this does the trick? Hurr
				log.Printf("[WARNING] STOPPING ORG SCHEDULE for: %s", org.Id)

				value.Lock()
				org, err := shuffle.GetOrg(ctx, org.Id)
				if err != nil {
					log.Printf("[WARNING] Failed finding org %s: %s", org.Id, err)
					return err
				}

				org.SyncConfig.Interval = 0
				org.SyncConfig.Apikey = ""
				org.CloudSync = false

				// Just in case
				org, err = handleStopCloudSync(syncUrl, *org)

				startDate := time.Now().Unix()
				org.SyncFeatures.Webhook = shuffle.SyncData{Active: false, Type: "trigger", Name: "Webhook", StartDate: startDate}
				org.SyncFeatures.UserInput = shuffle.SyncData{Active: false, Type: "trigger", Name: "User Input", StartDate: startDate}
				org.SyncFeatures.EmailTrigger = shuffle.SyncData{Active: false, Type: "action", Name: "Email Trigger", StartDate: startDate}
				org.SyncFeatures.Schedules = shuffle.SyncData{Active: false, Type: "trigger", Name: "Schedule", StartDate: startDate, Limit: 0}
				org.SyncFeatures.SendMail = shuffle.SyncData{Active: false, Type: "action", Name: "Send Email", StartDate: startDate, Limit: 0}
				org.SyncFeatures.SendSms = shuffle.SyncData{Active: false, Type: "action", Name: "Send SMS", StartDate: startDate, Limit: 0}
				org.CloudSyncActive = false

				err = shuffle.SetOrg(ctx, *org, org.Id)
				if err != nil {
					log.Printf("[WARNING] Failed setting organization when stopping sync: %s", err)
				} else {
					log.Printf("[INFO] Successfully STOPPED org cloud sync for %s", org.Id)
				}

				return errors.New("Stopped schedule for org locally because of bad apikey.")
			} else {
				return errors.New(fmt.Sprintf("Failed finding the schedule for org %s", org.Id))
			}
		}

		return errors.New("[ERROR] Remote job handler issues.")
	}

	if len(responseData.Jobs) > 0 {
		//log.Printf("[INFO] Remote JOB ret: %s", string(body))
		log.Printf("Got job with reason %s and %d job(s)", responseData.Reason, len(responseData.Jobs))
	}

	for _, job := range responseData.Jobs {
		err = handleCloudJob(job)
		if err != nil {
			log.Printf("[ERROR] Failed job from cloud: %s", err)
		}
	}

	return nil
}

func remoteOrgJobHandler(org shuffle.Org, interval int) error {
	client := &http.Client{}
	syncUrl := fmt.Sprintf("%s/api/v1/cloud/sync", syncUrl)
	req, err := http.NewRequest(
		"GET",
		syncUrl,
		nil,
	)

	req.Header.Add("Authorization", fmt.Sprintf(`Bearer %s`, org.SyncConfig.Apikey))
	newresp, err := client.Do(req)
	if err != nil {
		//log.Printf("Failed request in org sync: %s", err)
		return err
	}

	respBody, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		log.Printf("[ERROR] Failed body read in job sync: %s", err)
		return err
	}

	//log.Printf("Remote Data: %s", respBody)
	err = remoteOrgJobController(org, respBody)
	if err != nil {
		log.Printf("[ERROR] Failed job controller run for %s: %s", respBody, err)
		return err
	}
	return nil
}

// Handles configuration items during Shuffle startup
func runInit(ctx context.Context) {
	// Setting stats for backend starts (failure count as well)
	log.Printf("Starting INIT setup")
	err := increaseStatisticsField(ctx, "backend_executions", "", 1, "")
	if err != nil {
		log.Printf("Failed increasing local stats: %s", err)
	}
	log.Printf("Finalized init statistics update")

	httpProxy := os.Getenv("HTTP_PROXY")
	if len(httpProxy) > 0 {
		log.Printf("Running with HTTP proxy %s (env: HTTP_PROXY)", httpProxy)
	}
	httpsProxy := os.Getenv("HTTPS_PROXY")
	if len(httpsProxy) > 0 {
		log.Printf("Running with HTTPS proxy %s (env: HTTPS_PROXY)", httpsProxy)
	}

	//requestCache = cache.New(5*time.Minute, 10*time.Minute)

	/*
			proxyUrl, err := url.Parse(httpProxy)
			if err != nil {
				log.Printf("Failed setting up proxy: %s", err)
			} else {
				// accept any certificate (might be useful for testing)
				customClient := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
						Proxy:           http.ProxyURL(proxyUrl),
					},

					// 15 second timeout
					Timeout: 15 * 15time.Second,

					// don't follow redirect
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}

				// Override http(s) default protocol to use our custom client
				client.InstallProtocol("http", githttp.NewClient(customClient))
				client.InstallProtocol("https", githttp.NewClient(customClient))
			}
		}

		httpsProxy := os.Getenv("SHUFFLE_HTTPS_PROXY")
		if len(httpsProxy) > 0 {
			log.Printf("Running with HTTPS proxy %s", httpsProxy)
		}
	*/

	setUsers := false
	orgQuery := datastore.NewQuery("Organizations")
	var activeOrgs []shuffle.Org
	_, err = dbclient.GetAll(ctx, orgQuery, &activeOrgs)
	if err != nil {
		log.Printf("Error getting organizations!")
	} else {
		// Add all users to it
		if len(activeOrgs) == 1 {
			setUsers = true
		}

		log.Printf("Organizations exist!")
		if len(activeOrgs) == 0 {
			log.Printf(`No orgs. Setting org "default"`)
			orgSetupName := "default"
			orgId := uuid.NewV4().String()
			newOrg := shuffle.Org{
				Name:      orgSetupName,
				Id:        orgId,
				Org:       orgSetupName,
				Users:     []shuffle.User{},
				Roles:     []string{"admin", "user"},
				CloudSync: false,
			}

			err = shuffle.SetOrg(ctx, newOrg, orgId)
			if err != nil {
				log.Printf("Failed setting organization: %s", err)
			} else {
				log.Printf("Successfully created the default org!")
				setUsers = true
			}
		} else {
			log.Printf("There are %d org(s).", len(activeOrgs))
		}
	}

	// Adding the users to the base organization since only one exists (default)
	if setUsers && len(activeOrgs) > 0 {
		activeOrg := activeOrgs[0]

		q := datastore.NewQuery("Users")
		var users []shuffle.User
		_, err = dbclient.GetAll(ctx, q, &users)
		if err == nil {
			setOrgBool := false
			for _, user := range users {
				newUser := shuffle.User{
					Username: user.Username,
					Id:       user.Id,
					ActiveOrg: shuffle.Org{
						Id: activeOrg.Id,
					},
					Orgs: []string{activeOrg.Id},
					Role: user.Role,
				}

				found := false
				for _, orgUser := range activeOrg.Users {
					if user.Id == orgUser.Id {
						found = true
					}
				}

				if !found && len(user.Username) > 0 {
					log.Printf("Adding user %s to org %s", user.Username, activeOrg.Name)
					activeOrg.Users = append(activeOrg.Users, newUser)
					setOrgBool = true
				}
			}

			if setOrgBool {
				err = shuffle.SetOrg(ctx, activeOrg, activeOrg.Id)
				if err != nil {
					log.Printf("Failed setting org %s: %s!", activeOrg.Name, err)
				} else {
					log.Printf("UPDATED org %s!", activeOrg.Name)
				}
			}
		}

		log.Printf("Should add %d users to organization default", len(users))
	}

	if len(activeOrgs) == 0 {
		orgQuery := datastore.NewQuery("Organizations")
		_, err = dbclient.GetAll(ctx, orgQuery, &activeOrgs)
		if err != nil {
			log.Printf("Failed getting orgs the second time around")
		}
	}

	// Fix active users etc
	q := datastore.NewQuery("Users").Filter("active =", true)
	var activeusers []shuffle.User
	_, err = dbclient.GetAll(ctx, q, &activeusers)
	if err != nil {
		log.Printf("Error getting users during init: %s", err)
	} else {
		q := datastore.NewQuery("Users")
		var users []shuffle.User
		_, err := dbclient.GetAll(ctx, q, &users)

		if len(activeusers) == 0 && len(users) > 0 {
			log.Printf("No active users found - setting ALL to active")
			if err == nil {
				for _, user := range users {
					user.Active = true
					if len(user.Username) == 0 {
						DeleteKey(ctx, "Users", strings.ToLower(user.Username))
						continue
					}

					if len(user.Role) > 0 {
						user.Roles = append(user.Roles, user.Role)
					}

					if len(user.Orgs) == 0 {
						defaultName := "default"
						user.Orgs = []string{defaultName}
						user.ActiveOrg = shuffle.Org{
							Name: defaultName,
							Role: "user",
						}
					}

					err = shuffle.SetUser(ctx, &user)
					if err != nil {
						log.Printf("Failed to reset user")
					} else {
						log.Printf("Remade user %s with ID", user.Id)
						err = DeleteKey(ctx, "Users", strings.ToLower(user.Username))
						if err != nil {
							log.Printf("Failed to delete old user by username")
						}
					}
				}
			}
		} else if len(users) == 0 {
			log.Printf("Trying to set up user based on environments SHUFFLE_DEFAULT_USERNAME & SHUFFLE_DEFAULT_PASSWORD")
			username := os.Getenv("SHUFFLE_DEFAULT_USERNAME")
			password := os.Getenv("SHUFFLE_DEFAULT_PASSWORD")
			if len(username) == 0 || len(password) == 0 {
				log.Printf("SHUFFLE_DEFAULT_USERNAME and SHUFFLE_DEFAULT_PASSWORD not defined as environments. Running without default user.")
			} else {
				apikey := os.Getenv("SHUFFLE_DEFAULT_APIKEY")

				tmpOrg := shuffle.Org{
					Name: "default",
				}
				err = createNewUser(username, password, "admin", apikey, tmpOrg)
				if err != nil {
					log.Printf("Failed to create default user %s: %s", username, err)
				} else {
					log.Printf("Successfully created user %s", username)
				}
			}
		} else {
			if len(users) < 5 && len(users) > 0 {
				for _, user := range users {
					log.Printf("[INFO] Username: %s, role: %s", user.Username, user.Role)
				}
			} else {
				log.Printf("Found %d users.", len(users))
			}

			if len(activeOrgs) == 1 && len(users) > 0 {
				for _, user := range users {
					if user.ActiveOrg.Id == "" && len(user.Username) > 0 {
						user.ActiveOrg = activeOrgs[0]
						err = shuffle.SetUser(ctx, &user)
						if err != nil {
							log.Printf("Failed updating user %s with org", user.Username)
						} else {
							log.Printf("Updated user %s to have org", user.Username)
						}
					}
				}
			}
			//log.Printf(users[0].Username)
		}
	}

	// Gets environments and inits if it doesn't exist
	count, err := getEnvironmentCount()
	if count == 0 && err == nil && len(activeOrgs) == 1 {
		log.Printf("Setting up environment with org %s", activeOrgs[0].Id)
		item := shuffle.Environment{
			Name:    "Shuffle",
			Type:    "onprem",
			OrgId:   activeOrgs[0].Id,
			Default: true,
		}

		err = setEnvironment(ctx, &item)
		if err != nil {
			log.Printf("Failed setting up new environment")
		}
	} else if len(activeOrgs) == 1 {
		log.Printf("Setting up all environments with org %s", activeOrgs[0].Id)
		var environments []shuffle.Environment
		q := datastore.NewQuery("Environments")
		_, err = dbclient.GetAll(ctx, q, &environments)
		if err == nil {
			for _, item := range environments {
				if item.OrgId == activeOrgs[0].Id {
					continue
				}

				item.OrgId = activeOrgs[0].Id
				err = setEnvironment(ctx, &item)
				if err != nil {
					log.Printf("Failed adding environment to org %s", activeOrgs[0].Id)
				}
			}
		}
	}

	// Fixing workflows to have real activeorg IDs
	if len(activeOrgs) == 1 {
		q := datastore.NewQuery("workflow").Limit(35)
		var workflows []shuffle.Workflow
		_, err = dbclient.GetAll(ctx, q, &workflows)
		if err != nil {
			log.Printf("Error getting workflows in runinit: %s", err)
		} else {
			updated := 0
			timeNow := time.Now().Unix()
			for _, workflow := range workflows {
				setLocal := false
				if workflow.ExecutingOrg.Id == "" || len(workflow.OrgId) == 0 {
					workflow.OrgId = activeOrgs[0].Id
					workflow.ExecutingOrg = activeOrgs[0]
					setLocal = true
				} else if workflow.Edited == 0 {
					workflow.Edited = timeNow
					setLocal = true
				}

				if setLocal {
					err = shuffle.SetWorkflow(ctx, workflow, workflow.ID)
					if err != nil {
						log.Printf("Failed setting workflow in init: %s", err)
					} else {
						log.Printf("Fixed workflow %s to have the right info.", workflow.ID)
						updated += 1
					}
				}
			}

			if updated > 0 {
				log.Printf("Set workflow orgs for %d workflows", updated)
			}
		}

		/*
			fileq := datastore.NewQuery("Files").Limit(1)
			count, err := dbclient.Count(ctx, fileq)
			log.Printf("FILECOUNT: %d", count)
			if err == nil && count < 10 {
				basepath := "."
				filename := "testfile.txt"
				fileId := uuid.NewV4().String()
				log.Printf("Creating new file reference %s because none exist!", fileId)
				workflowId := "2cf1169d-b460-41de-8c36-28b2092866f8"
				downloadPath := fmt.Sprintf("%s/%s/%s/%s", basepath, activeOrgs[0].Id, workflowId, fileId)

				timeNow := time.Now().Unix()
				newFile := File{
					Id:           fileId,
					CreatedAt:    timeNow,
					UpdatedAt:    timeNow,
					Description:  "Created by system for testing",
					Status:       "active",
					Filename:     filename,
					OrgId:        activeOrgs[0].Id,
					WorkflowId:   workflowId,
					DownloadPath: downloadPath,
				}

				err = setFile(ctx, newFile)
				if err != nil {
					log.Printf("Failed setting file: %s", err)
				} else {
					log.Printf("Created file %s in init", newFile.DownloadPath)
				}
			}
		*/

		var allworkflowapps []shuffle.AppAuthenticationStorage
		q = datastore.NewQuery("workflowappauth")
		_, err = dbclient.GetAll(ctx, q, &allworkflowapps)
		if err == nil {
			log.Printf("Setting up all app auths with org %s", activeOrgs[0].Id)
			for _, item := range allworkflowapps {
				if item.OrgId != "" {
					continue
				}

				//log.Printf("Should update auth for %#v!", item)
				item.OrgId = activeOrgs[0].Id
				err = shuffle.SetWorkflowAppAuthDatastore(ctx, item, item.Id)
				if err != nil {
					log.Printf("Failed adding AUTH to org %s", activeOrgs[0].Id)
				}
			}
		}

		var schedules []ScheduleOld
		q = datastore.NewQuery("schedules")
		_, err = dbclient.GetAll(ctx, q, &schedules)
		if err == nil {
			log.Printf("Setting up all schedules with org %s", activeOrgs[0].Id)
			for _, item := range schedules {
				if item.Org != "" {
					continue
				}

				log.Printf("ENV: %s", item.Environment)
				if item.Environment == "cloud" {
					log.Printf("Skipping cloud schedule")
					continue
				}

				item.Org = activeOrgs[0].Id
				err = setSchedule(ctx, item)
				if err != nil {
					log.Printf("Failed adding schedule to org %s", activeOrgs[0].Id)
				}
			}
		}
	}

	log.Printf("Starting cloud schedules for orgs!")
	type requestStruct struct {
		ApiKey string `json:"api_key"`
	}
	for _, org := range activeOrgs {
		if !org.CloudSync {
			log.Printf("Skipping org %s because sync isn't set (1).", org.Id)
			continue
		}

		//interval := int(org.SyncConfig.Interval)
		interval := 15
		if interval == 0 {
			log.Printf("Skipping org %s because sync isn't set (0).", org.Id)
			continue
		}

		log.Printf("Should start schedule for org %s", org.Name)
		job := func() {
			err := remoteOrgJobHandler(org, interval)
			if err != nil {
				log.Printf("[ERROR] Failed request with remote org setup (2): %s", err)
			}
		}

		jobret, err := newscheduler.Every(int(interval)).Seconds().NotImmediately().Run(job)
		if err != nil {
			log.Printf("[CRITICAL] Failed to schedule org: %s", err)
		} else {
			log.Printf("Started sync on interval %d for org %s", interval, org.Name)
			scheduledOrgs[org.Id] = jobret
		}
	}

	// Gets schedules and starts them
	log.Printf("Relaunching schedules")
	schedules, err := getAllSchedules(ctx, "ALL")
	if err != nil {
		log.Printf("Failed getting schedules during service init: %s", err)
	} else {
		log.Printf("Setting up %d schedule(s)", len(schedules))
		url := &url.URL{}
		for _, schedule := range schedules {
			if schedule.Environment == "cloud" {
				log.Printf("Skipping cloud schedule")
				continue
			}

			//log.Printf("Schedule: %#v", schedule)
			job := func() {
				request := &http.Request{
					URL:    url,
					Method: "POST",
					Body:   ioutil.NopCloser(strings.NewReader(schedule.WrappedArgument)),
				}

				_, _, err := handleExecution(schedule.WorkflowId, shuffle.Workflow{}, request)
				if err != nil {
					log.Printf("Failed to execute %s: %s", schedule.WorkflowId, err)
				}
			}

			//log.Printf("Schedule time: every %d seconds", schedule.Seconds)
			jobret, err := newscheduler.Every(schedule.Seconds).Seconds().NotImmediately().Run(job)
			if err != nil {
				log.Printf("Failed to schedule workflow: %s", err)
			}

			scheduledJobs[schedule.Id] = jobret
		}
	}

	// form force-flag to download workflow apps
	forceUpdateEnv := os.Getenv("SHUFFLE_APP_FORCE_UPDATE")
	forceUpdate := false
	if len(forceUpdateEnv) > 0 && forceUpdateEnv == "true" {
		log.Printf("Forcing to rebuild apps")
		forceUpdate = true
	}

	// Getting apps to see if we should initialize a test
	log.Printf("Getting remote workflow apps")
	workflowapps, err := shuffle.GetAllWorkflowApps(ctx, 500)
	if err != nil {
		log.Printf("Failed getting apps (runInit): %s", err)
	} else if err == nil && len(workflowapps) > 0 {
		var allworkflowapps []shuffle.WorkflowApp
		q := datastore.NewQuery("workflowapp")
		_, err := dbclient.GetAll(ctx, q, &allworkflowapps)
		if err == nil {
			for _, workflowapp := range allworkflowapps {
				if workflowapp.Edited == 0 {
					err = shuffle.SetWorkflowAppDatastore(ctx, workflowapp, workflowapp.ID)
					if err == nil {
						log.Printf("Updating time for workflowapp %s:%s", workflowapp.Name, workflowapp.AppVersion)
					}
				}
			}
		}

	} else if err == nil && len(workflowapps) == 0 {
		log.Printf("Downloading default workflow apps")
		fs := memfs.New()
		storer := memory.NewStorage()

		url := os.Getenv("SHUFFLE_APP_DOWNLOAD_LOCATION")
		if len(url) == 0 {
			url = "https://github.com/frikky/shuffle-apps"
		}

		username := os.Getenv("SHUFFLE_DOWNLOAD_AUTH_USERNAME")
		password := os.Getenv("SHUFFLE_DOWNLOAD_AUTH_PASSWORD")

		cloneOptions := &git.CloneOptions{
			URL: url,
		}

		if len(username) > 0 && len(password) > 0 {
			cloneOptions.Auth = &http2.BasicAuth{
				Username: username,
				Password: password,
			}
		}
		branch := os.Getenv("SHUFFLE_DOWNLOAD_AUTH_BRANCH")
		if len(branch) > 0 && branch != "master" && branch != "main" {
			cloneOptions.ReferenceName = plumbing.ReferenceName(branch)
		}

		log.Printf("Getting apps from %s", url)

		r, err := git.Clone(storer, fs, cloneOptions)

		if err != nil {
			log.Printf("Failed loading repo into memory (init): %s", err)
		}

		dir, err := fs.ReadDir("")
		if err != nil {
			log.Printf("Failed reading folder: %s", err)
		}
		_ = r
		//iterateAppGithubFolders(fs, dir, "", "testing")

		// FIXME: Get all the apps?
		_, _, err = iterateAppGithubFolders(fs, dir, "", "", forceUpdate)
		if err != nil {
			log.Printf("[WARNING] Error from app load in init: %s", err)
		}
		//_, _, err = iterateAppGithubFolders(fs, dir, "", "", forceUpdate)

		// Hotloads locally
		location := os.Getenv("SHUFFLE_APP_HOTLOAD_FOLDER")
		if len(location) != 0 {
			handleAppHotload(ctx, location, false)
		}
	}

	log.Printf("[INFO] Downloading OpenAPI data for search - EXTRA APPS")
	apis := "https://github.com/frikky/security-openapis"

	// THis gets memory problems hahah
	//apis := "https://github.com/APIs-guru/openapi-directory"
	fs := memfs.New()
	storer := memory.NewStorage()
	cloneOptions := &git.CloneOptions{
		URL: apis,
	}
	_, err = git.Clone(storer, fs, cloneOptions)
	if err != nil {
		log.Printf("Failed loading repo %s into memory: %s", apis, err)
	} else {
		log.Printf("[INFO] Finished git clone. Looking for updates to the repo.")
		dir, err := fs.ReadDir("")
		if err != nil {
			log.Printf("Failed reading folder: %s", err)
		}

		iterateOpenApiGithub(fs, dir, "", "")
		log.Printf("[INFO] Finished downloading extra API samples")
	}

	workflowLocation := os.Getenv("SHUFFLE_DOWNLOAD_WORKFLOW_LOCATION")
	if len(workflowLocation) > 0 {
		log.Printf("[INFO] Downloading WORKFLOWS from %s if no workflows - EXTRA workflows", workflowLocation)
		q := datastore.NewQuery("workflow").Limit(35)
		var workflows []shuffle.Workflow
		_, err = dbclient.GetAll(ctx, q, &workflows)
		if err != nil {
			log.Printf("Error getting workflows: %s", err)
		} else {
			if len(workflows) == 0 {
				username := os.Getenv("SHUFFLE_DOWNLOAD_WORKFLOW_USERNAME")
				password := os.Getenv("SHUFFLE_DOWNLOAD_WORKFLOW_PASSWORD")
				orgId := ""
				if len(activeOrgs) > 0 {
					orgId = activeOrgs[0].Id
				}

				err = loadGithubWorkflows(workflowLocation, username, password, "", os.Getenv("SHUFFLE_DOWNLOAD_WORKFLOW_BRANCH"), orgId)
				if err != nil {
					log.Printf("Failed to upload workflows from github: %s", err)
				} else {
					log.Printf("[INFO] Finished downloading workflows from github!")
				}
			} else {
				log.Printf("[INFO] Skipping because there are %d workflows already", len(workflows))
			}

		}
	}

	log.Printf("[INFO] Finished INIT")
}

func handleVerifyCloudsync(orgId string) (shuffle.SyncFeatures, error) {
	ctx := context.Background()
	org, err := shuffle.GetOrg(ctx, orgId)
	if err != nil {
		return shuffle.SyncFeatures{}, err
	}

	//r.HandleFunc("/api/v1/getorgs", handleGetOrgs).Methods("GET", "OPTIONS")

	syncURL := fmt.Sprintf("%s/api/v1/cloud/sync/get_access", syncUrl)
	client := &http.Client{}
	req, err := http.NewRequest(
		"GET",
		syncURL,
		nil,
	)

	req.Header.Add("Authorization", fmt.Sprintf(`Bearer %s`, org.SyncConfig.Apikey))
	newresp, err := client.Do(req)
	if err != nil {
		return shuffle.SyncFeatures{}, err
	}

	respBody, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		return shuffle.SyncFeatures{}, err
	}

	responseData := retStruct{}
	err = json.Unmarshal(respBody, &responseData)
	if err != nil {
		return shuffle.SyncFeatures{}, err
	}

	if newresp.StatusCode != 200 {
		return shuffle.SyncFeatures{}, errors.New(fmt.Sprintf("Got status code %d when getting org remotely. Expected 200. Contact support.", newresp.StatusCode))
	}

	if !responseData.Success {
		return shuffle.SyncFeatures{}, errors.New(responseData.Reason)
	}

	return responseData.SyncFeatures, nil
}

// Actually stops syncing with cloud for an org.
// Disables potential schedules, removes environments, breaks workflows etc.
func handleStopCloudSync(syncUrl string, org shuffle.Org) (*shuffle.Org, error) {
	if len(org.SyncConfig.Apikey) == 0 {
		return &org, errors.New(fmt.Sprintf("Couldn't find any sync key to disable org %s", org.Id))
	}

	log.Printf("Should run cloud sync disable for org %s with URL %s and sync key %s", org.Id, syncUrl, org.SyncConfig.Apikey)

	client := &http.Client{}
	req, err := http.NewRequest(
		"DELETE",
		syncUrl,
		nil,
	)

	req.Header.Add("Authorization", fmt.Sprintf(`Bearer %s`, org.SyncConfig.Apikey))
	newresp, err := client.Do(req)
	if err != nil {
		return &org, err
	}

	respBody, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		return &org, err
	}
	log.Printf("Remote disable ret: %s", string(respBody))

	responseData := retStruct{}
	err = json.Unmarshal(respBody, &responseData)
	if err != nil {
		return &org, err
	}

	if newresp.StatusCode != 200 {
		return &org, errors.New(fmt.Sprintf("Got status code %d when disabling org remotely. Expected 200. Contact support.", newresp.StatusCode))
	}

	if !responseData.Success {
		//log.Printf("Success reason: %s", responseData.Reason)
		return &org, errors.New(responseData.Reason)
	}

	log.Printf("Everything is success. Should disable org sync for %s", org.Id)

	ctx := context.Background()
	org.CloudSync = false
	org.SyncFeatures = shuffle.SyncFeatures{}
	org.SyncConfig = shuffle.SyncConfig{}

	err = shuffle.SetOrg(ctx, org, org.Id)
	if err != nil {
		newerror := fmt.Sprintf("ERROR: Failed updating even though there was success: %s", err)
		log.Printf(newerror)
		return &org, errors.New(newerror)
	}

	var environments []shuffle.Environment
	q := datastore.NewQuery("Environments").Filter("org_id =", org.Id)
	_, err = dbclient.GetAll(ctx, q, &environments)
	if err != nil {
		return &org, err
	}

	// Don't disable, this will be deleted entirely
	for _, environment := range environments {
		if environment.Type == "cloud" {
			environment.Name = "Cloud"
			environment.Archived = true
			err = setEnvironment(ctx, &environment)
			if err == nil {
				log.Printf("[INFO] Updated cloud environment %s", environment.Name)
			} else {
				log.Printf("[INFO] Failed to update cloud environment %s", environment.Name)
			}
		}
	}

	// FIXME: This doesn't work?
	if value, exists := scheduledOrgs[org.Id]; exists {
		// Looks like this does the trick? Hurr
		log.Printf("[WARNING] STOPPING ORG SCHEDULE for: %s", org.Id)

		value.Lock()
	}

	return &org, nil
}

// INFO: https://docs.google.com/drawings/d/1JJebpPeEVEbmH_qsAC6zf9Noygp7PytvesrkhE19QrY/edit
/*
	This is here to both enable and disable cloud sync features for an organization
*/
func handleCloudSetup(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in cloud setup: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Role != "admin" {
		log.Printf("Not admin.")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Not admin"}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Failed reading body"}`))
		return
	}

	type ReturnData struct {
		Apikey       string      `datastore:"apikey"`
		Organization shuffle.Org `datastore:"organization"`
		Disable      bool        `datastore:"disable"`
	}

	var tmpData ReturnData
	err = json.Unmarshal(body, &tmpData)
	if err != nil {
		log.Printf("Failed unmarshalling test: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	ctx := context.Background()
	org, err := shuffle.GetOrg(ctx, tmpData.Organization.Id)
	if err != nil {
		log.Printf("Organization doesn't exist: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME: Check if user is admin of this org
	//log.Printf("Checking org %s", org.Name)
	userFound := false
	admin := false
	for _, inneruser := range org.Users {
		if inneruser.Id == user.Id {
			userFound = true
			//log.Printf("[INFO] Role: %s", inneruser.Role)
			if inneruser.Role == "admin" {
				admin = true
			}

			break
		}
	}

	if !userFound {
		log.Printf("User %s doesn't exist in organization %s", user.Id, org.Id)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME: Enable admin check in org for sync setup and conf.
	_ = admin
	//if !admin {
	//	log.Printf("User %s isn't admin hence can't set up sync for org %s", user.Id, org.Id)
	//	resp.WriteHeader(401)
	//	resp.Write([]byte(`{"success": false}`))
	//	return
	//}

	//log.Printf("Apidata: %s", tmpData.Apikey)

	// FIXME: Path
	client := &http.Client{}
	apiPath := "/api/v1/cloud/sync/setup"
	if tmpData.Disable {
		if !org.CloudSync {
			log.Printf("[WARNING] Org %s isn't syncing. Can't stop.", org.Id)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Skipped cloud sync setup. Already syncing."}`)))
			return
		}

		log.Printf("[INFO] Should disable sync for org %s", org.Id)
		apiPath := "/api/v1/cloud/sync/stop"
		syncPath := fmt.Sprintf("%s%s", syncUrl, apiPath)

		_, err = handleStopCloudSync(syncPath, *org)
		if err != nil {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		} else {
			resp.WriteHeader(200)
			resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "Successfully disabled cloud sync for org."}`)))
		}

		return
	}

	// Everything below here is to SET UP CLOUD SYNC.
	// If you want to disable cloud sync, see previous section.
	if org.CloudSync {
		log.Printf("Org %s is already syncing. Skip", org.Id)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Org is already syncing. Nothing to set up."}`)))
		return
	}

	syncPath := fmt.Sprintf("%s%s", syncUrl, apiPath)

	type requestStruct struct {
		ApiKey string `json:"api_key"`
	}

	requestData := requestStruct{
		ApiKey: tmpData.Apikey,
	}

	b, err := json.Marshal(requestData)
	if err != nil {
		log.Printf("Failed marshaling api key data: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed cloud sync: %s"}`, err)))
		return
	}

	req, err := http.NewRequest(
		"POST",
		syncPath,
		bytes.NewBuffer(b),
	)

	newresp, err := client.Do(req)
	if err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed cloud sync: %s. Contact support."}`, err)))
		//setBadMemcache(ctx, docPath)
		return
	}

	respBody, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Can't parse sync data. Contact support."}`)))
		return
	}

	//log.Printf("Respbody: %s", string(respBody))
	responseData := retStruct{}
	err = json.Unmarshal(respBody, &responseData)
	if err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed handling cloud data"}`)))
		return
	}

	if newresp.StatusCode != 200 {
		resp.WriteHeader(401)
		resp.Write(respBody)
		return
	}

	if !responseData.Success {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, responseData.Reason)))
		return
	}

	// FIXME:
	// 1. Set cloudsync for org to be active
	// 2. Add iterative sync schedule for interval seconds
	// 3. Add another environment for the org's users
	org.CloudSync = true
	org.SyncFeatures = responseData.SyncFeatures

	org.SyncConfig = shuffle.SyncConfig{
		Apikey:   responseData.SessionKey,
		Interval: responseData.IntervalSeconds,
	}

	interval := int(responseData.IntervalSeconds)
	log.Printf("[INFO] Starting cloud sync on interval %d", interval)
	job := func() {
		err := remoteOrgJobHandler(*org, interval)
		if err != nil {
			log.Printf("[ERROR] Failed request with remote org setup (1): %s", err)
		}
	}

	jobret, err := newscheduler.Every(int(interval)).Seconds().NotImmediately().Run(job)
	if err != nil {
		log.Printf("[CRITICAL] Failed to schedule org: %s", err)
	} else {
		log.Printf("[INFO] Started sync on interval %d for org %s", interval, org.Name)
		scheduledOrgs[org.Id] = jobret
	}

	// FIXME: Add this for every feature
	if org.SyncFeatures.Workflows.Active {
		log.Printf("[INFO] Should activate cloud workflows for org %s!", org.Id)

		// 1. Find environment
		// 2. If cloud env found, enable it (un-archive)
		// 3. If it doesn't create it
		var environments []shuffle.Environment
		q := datastore.NewQuery("Environments").Filter("org_id =", org.Id)
		_, err = dbclient.GetAll(ctx, q, &environments)
		if err == nil {

			// Don't disable, this will be deleted entirely
			found := false
			for _, environment := range environments {
				if environment.Type == "cloud" {
					environment.Name = "Cloud"
					environment.Archived = false
					err = setEnvironment(ctx, &environment)
					if err == nil {
						log.Printf("[INFO] Re-added cloud environment %s", environment.Name)
					} else {
						log.Printf("[INFO] Failed to re-enable cloud environment %s", environment.Name)
					}

					found = true
					break
				}
			}

			if !found {
				log.Printf("[INFO] Env for cloud not found. Should add it!")
				newEnv := shuffle.Environment{
					Name:       "Cloud",
					Type:       "cloud",
					Archived:   false,
					Registered: true,
					Default:    false,
					OrgId:      org.Id,
				}

				err = setEnvironment(ctx, &newEnv)
				if err != nil {
					log.Printf("Failed setting up NEW org environment for org %s: %s", org.Id, err)
				} else {
					log.Printf("Successfully added new environment for org %s", org.Id)
				}
			}
		} else {
			log.Printf("Failed setting org environment, because none were found: %s", err)
		}
	}

	err = shuffle.SetOrg(ctx, *org, org.Id)
	if err != nil {
		log.Printf("ERROR: Failed updating org even though there was success: %s", err)
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed setting up org after sync success. Contact support."}`)))
		return
	}

	if responseData.IntervalSeconds > 0 {
		// FIXME:
		log.Printf("[INFO] Should set up interval for %d with session key %s for org %s", responseData.IntervalSeconds, responseData.SessionKey, org.Name)
	}

	resp.WriteHeader(200)
	resp.Write(respBody)
}

func makeWorkflowPublic(resp http.ResponseWriter, request *http.Request) {
	cors := shuffle.HandleCors(resp, request)
	if cors {
		return
	}

	user, userErr := shuffle.HandleApiAuthentication(resp, request)
	if userErr != nil {
		log.Printf("[WARNING] Api authentication failed in make workflow public: %s", userErr)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
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

	ctx := context.Background()
	if strings.Contains(fileId, "?") {
		fileId = strings.Split(fileId, "?")[0]
	}

	if len(fileId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID when getting workflow is not valid"}`))
		return
	}

	workflow, err := shuffle.GetWorkflow(ctx, fileId)
	if err != nil {
		log.Printf("[WARNING] Workflow %s doesn't exist in app publish. User: %s", fileId, user.Id)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// CHECK orgs of user, or if user is owner
	// FIXME - add org check too, and not just owner
	// Check workflow.Sharing == private / public / org  too
	if user.Id != workflow.Owner || len(user.Id) == 0 {
		if workflow.OrgId == user.ActiveOrg.Id && user.Role == "admin" {
			log.Printf("[INFO] User %s is accessing workflow %s as admin", user.Username, workflow.ID)
		} else {
			log.Printf("[WARNING] Wrong user (%s) for workflow %s (get workflow)", user.Username, workflow.ID)
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}
	}

	if !workflow.IsValid || !workflow.PreviouslySaved {
		log.Printf("[INFO] Failed uploading workflow because it's invalid or not saved")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Invalid workflows are not sharable"}`))
		return
	}

	// Starting validation of the POST workflow
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("[WARNING] Body data error on mail: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	parsedWorkflow := shuffle.Workflow{}
	err = json.Unmarshal(body, &parsedWorkflow)
	if err != nil {
		log.Printf("[WARNING] Unmarshal error on mail: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Super basic validation. Doesn't really matter.
	if parsedWorkflow.ID != workflow.ID || len(parsedWorkflow.Actions) != len(workflow.Actions) {
		log.Printf("[WARNING] Bad ID during publish: %s vs %s", workflow.ID, parsedWorkflow.ID)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if !workflow.IsValid || !workflow.PreviouslySaved {
		log.Printf("[INFO] Failed uploading new workflow because it's invalid or not saved")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Invalid workflows are not sharable"}`))
		return
	}

	workflowData, err := json.Marshal(parsedWorkflow)
	if err != nil {
		log.Printf("[WARNING] Failed marshalling workflow: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Sanitization is done in the frontend as well
	parsedWorkflow = shuffle.SanitizeWorkflow(parsedWorkflow)
	parsedWorkflow.ID = uuid.NewV4().String()
	action := shuffle.CloudSyncJob{
		Type:          "workflow",
		Action:        "publish",
		OrgId:         user.ActiveOrg.Id,
		PrimaryItemId: workflow.ID,
		SecondaryItem: string(workflowData),
		FifthItem:     user.Id,
	}

	err = executeCloudAction(action, user.ActiveOrg.SyncConfig.Apikey)
	if err != nil {
		log.Printf("[WARNING] Failed cloud PUBLISH: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	log.Printf("[INFO] Successfully published workflow %s (%s) TO CLOUD", workflow.Name, workflow.ID)
	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

func initHandlers() {
	var err error
	ctx := context.Background()

	log.Printf("Starting Shuffle backend - initializing database connection")
	//requestCache = cache.New(5*time.Minute, 10*time.Minute)
	dbclient, err = datastore.NewClient(ctx, gceProject, option.WithGRPCDialOption(grpc.WithNoProxy()))
	if err != nil {
		panic(fmt.Sprintf("DBclient error during init: %s", err))
	}

	//dbclient, err := shuffle.GetDatastoreClient(ctx, gceProject)
	//if err != nil {
	//	panic(fmt.Sprintf("Error setting datastore connector: %s", err))
	//}

	_ = shuffle.RunInit(*dbclient, storage.Client{}, gceProject, "onprem", true)
	log.Printf("Finished Shuffle database init")

	go runInit(ctx)

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/_ah/health", healthCheckHandler)

	// Make user related locations
	// Fix user changes with org
	r.HandleFunc("/api/v1/users/login", handleLogin).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/users/register", handleRegister).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/users/checkusers", checkAdminLogin).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/users/getinfo", handleInfo).Methods("GET", "OPTIONS")

	r.HandleFunc("/api/v1/users/generateapikey", shuffle.HandleApiGeneration).Methods("GET", "POST", "OPTIONS")
	r.HandleFunc("/api/v1/users/logout", shuffle.HandleLogout).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/users/getsettings", shuffle.HandleSettings).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/users/getusers", shuffle.HandleGetUsers).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/users/updateuser", handleUpdateUser).Methods("PUT", "OPTIONS")
	r.HandleFunc("/api/v1/users/{user}", shuffle.DeleteUser).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/v1/users/passwordchange", shuffle.HandlePasswordChange).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/users", shuffle.HandleGetUsers).Methods("GET", "OPTIONS")

	// General - duplicates and old.
	r.HandleFunc("/api/v1/getusers", shuffle.HandleGetUsers).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/login", handleLogin).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/logout", shuffle.HandleLogout).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/register", handleRegister).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/checkusers", checkAdminLogin).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/getinfo", handleInfo).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/getsettings", shuffle.HandleSettings).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/generateapikey", shuffle.HandleApiGeneration).Methods("GET", "POST", "OPTIONS")
	r.HandleFunc("/api/v1/passwordchange", shuffle.HandlePasswordChange).Methods("POST", "OPTIONS")

	r.HandleFunc("/api/v1/getenvironments", shuffle.HandleGetEnvironments).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/setenvironments", shuffle.HandleSetEnvironments).Methods("PUT", "OPTIONS")

	r.HandleFunc("/api/v1/docs", getDocList).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/docs/{key}", getDocs).Methods("GET", "OPTIONS")

	// Queuebuilder and Workflow streams. First is to update a stream, second to get a stream
	// Changed from workflows/streams to streams, as appengine was messing up
	// This does not increase the API counter
	// Used by frontend
	r.HandleFunc("/api/v1/streams", handleWorkflowQueue).Methods("POST")
	r.HandleFunc("/api/v1/streams/results", handleGetStreamResults).Methods("POST", "OPTIONS")

	// Used by orborus
	r.HandleFunc("/api/v1/workflows/queue", handleGetWorkflowqueue).Methods("GET")
	r.HandleFunc("/api/v1/workflows/queue/confirm", handleGetWorkflowqueueConfirm).Methods("POST")

	// App specific
	// From here down isnt checked for org specific
	r.HandleFunc("/api/v1/apps/{appId}", shuffle.UpdateWorkflowAppConfig).Methods("PATCH", "OPTIONS")
	r.HandleFunc("/api/v1/apps/{appId}", shuffle.DeleteWorkflowApp).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/v1/apps/run_hotload", handleAppHotloadRequest).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/apps/get_existing", loadSpecificApps).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/apps/download_remote", loadSpecificApps).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/apps/validate", validateAppInput).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/apps/{appId}/config", getWorkflowAppConfig).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/apps", getWorkflowApps).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/apps", setNewWorkflowApp).Methods("PUT", "OPTIONS")
	r.HandleFunc("/api/v1/apps/search", getSpecificApps).Methods("POST", "OPTIONS")

	r.HandleFunc("/api/v1/apps/authentication", shuffle.GetAppAuthentication).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/apps/authentication", shuffle.AddAppAuthentication).Methods("PUT", "OPTIONS")
	r.HandleFunc("/api/v1/apps/authentication/{appauthId}/config", shuffle.SetAuthenticationConfig).Methods("POST", "OPTIONS")

	r.HandleFunc("/api/v1/apps/authentication/{appauthId}", shuffle.DeleteAppAuthentication).Methods("DELETE", "OPTIONS")

	// Legacy app things
	r.HandleFunc("/api/v1/workflows/apps/validate", validateAppInput).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/apps", getWorkflowApps).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/apps", setNewWorkflowApp).Methods("PUT", "OPTIONS")

	// Workflows
	// FIXME - implement the queue counter lol
	/* Everything below here increases the counters*/
	r.HandleFunc("/api/v1/workflows", shuffle.GetWorkflows).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/workflows", shuffle.SetNewWorkflow).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}", shuffle.GetSpecificWorkflow).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}", shuffle.SaveWorkflow).Methods("PUT", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/schedules", handleGetSchedules).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}/schedule", scheduleWorkflow).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/download_remote", loadSpecificWorkflows).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}/execute", executeWorkflow).Methods("GET", "POST", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}/schedule/{schedule}", stopSchedule).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}/outlook", createOutlookSub).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}/outlook/{triggerId}", handleDeleteOutlookSub).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}/executions", getWorkflowExecutions).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}/executions/{key}/abort", shuffle.AbortExecution).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/workflows/{key}", deleteWorkflow).Methods("DELETE", "OPTIONS")

	// Triggers
	r.HandleFunc("/api/v1/hooks/new", shuffle.HandleNewHook).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/hooks/{key}", handleWebhookCallback).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/hooks/{key}/delete", shuffle.HandleDeleteHook).Methods("DELETE", "OPTIONS")

	// OpenAPI configuration
	r.HandleFunc("/api/v1/verify_swagger", verifySwagger).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/verify_openapi", verifySwagger).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/get_openapi_uri", echoOpenapiData).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/validate_openapi", validateSwagger).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/get_openapi/{key}", getOpenapi).Methods("GET", "OPTIONS")

	// NEW for 0.8.0
	r.HandleFunc("/api/v1/workflows/{key}/publish", makeWorkflowPublic).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/cloud/setup", handleCloudSetup).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/orgs", shuffle.HandleGetOrgs).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/orgs/", shuffle.HandleGetOrgs).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/orgs/{orgId}", shuffle.HandleGetOrg).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/orgs/{orgId}", shuffle.HandleEditOrg).Methods("POST", "OPTIONS")
	// This is a new API that validates if a key has been seen before.
	// Not sure what the best course of action is for it.
	r.HandleFunc("/api/v1/orgs/{orgId}/validate_app_values", shuffle.HandleKeyValueCheck).Methods("POST", "OPTIONS")

	//r.HandleFunc("/api/v1/orgs/{orgId}", handleEditOrg).Methods("POST", "OPTIONS")

	// Docker orborus specific
	//r.HandleFunc("/api/v1/get_docker_image", getDockerImage).Methods("POST", "OPTIONS")

	// Important for email, IDS etc. Create this by:
	// PS: For cloud, this has to use cloud storage.
	// https://developer.box.com/reference/get-files-id-content/
	// 1. Creating the "get file" option. Make it possible to run this in the frontend.
	r.HandleFunc("/api/v1/files/{fileId}/content", shuffle.HandleGetFileContent).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/files/create", shuffle.HandleCreateFile).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/files/{fileId}/upload", shuffle.HandleUploadFile).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/files/{fileId}", shuffle.HandleGetFileMeta).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/files/{fileId}", shuffle.HandleDeleteFile).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/v1/files", shuffle.HandleGetFiles).Methods("GET", "OPTIONS")

	// Trigger hmm
	r.HandleFunc("/api/v1/triggers/outlook/register", handleNewOutlookRegister).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/triggers/outlook/getFolders", handleGetOutlookFolders).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/v1/triggers/outlook/{key}", handleGetSpecificTrigger).Methods("GET", "OPTIONS")
	//r.HandleFunc("/api/v1/triggers/outlook/{key}/callback", handleOutlookCallback).Methods("POST", "OPTIONS")
	//r.HandleFunc("/api/v1/stats/{key}", handleGetSpecificStats).Methods("GET", "OPTIONS")

	http.Handle("/", r)
}

// Had to move away from mux, which means Method is fucked up right now.
func main() {
	initHandlers()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "MISSING"
	}

	innerPort := os.Getenv("BACKEND_PORT")
	if innerPort == "" {
		log.Printf("Running on %s:5001", hostname)
		log.Fatal(http.ListenAndServe(":5001", nil))
	} else {
		log.Printf("Running on %s:%s", hostname, innerPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", innerPort), nil))
	}
}

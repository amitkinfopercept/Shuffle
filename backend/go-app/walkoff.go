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
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	"cloud.google.com/go/datastore"
	scheduler "cloud.google.com/go/scheduler/apiv1"
	gyaml "github.com/ghodss/yaml"
	"github.com/h2non/filetype"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/api/cloudfunctions/v1"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"

	newscheduler "github.com/carlescere/scheduler"
	"github.com/frikky/kin-openapi/openapi3"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	http2 "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	//"github.com/gorilla/websocket"
	//"google.golang.org/appengine"
	//"google.golang.org/appengine/memcache"
	//"cloud.google.com/go/firestore"
	// "google.golang.org/api/option"

	"google.golang.org/api/iterator"
)

var localBase = "http://localhost:5001"
var baseEnvironment = "onprem"

var cloudname = "cloud"
var defaultLocation = "europe-west2"
var scheduledJobs = map[string]*newscheduler.Job{}
var scheduledOrgs = map[string]*newscheduler.Job{}

// To test out firestore before potential merge
//var upgrader = websocket.Upgrader{
//	ReadBufferSize:  1024,
//	WriteBufferSize: 1024,
//	CheckOrigin: func(r *http.Request) bool {
//		return true
//	},
//}

//type ExecutionRequest struct {
//	ExecutionId       string   `json:"execution_id,omitempty"`
//	ExecutionArgument string   `json:"execution_argument,omitempty"`
//	ExecutionSource   string   `json:"execution_source,omitempty"`
//	WorkflowId        string   `json:"workflow_id,omitempty"`
//	Environments      []string `json:"environments,omitempty"`
//	Authorization     string   `json:"authorization,omitempty"`
//	Status            string   `json:"status,omitempty"`
//	Start             string   `json:"start,omitempty"`
//	Type              string   `json:"type,omitempty"`
//}
//
//type SyncFeatures struct {
//	Webhook            SyncData `json:"webhook" datastore:"webhook"`
//	Schedules          SyncData `json:"schedules" datastore:"schedules"`
//	UserInput          SyncData `json:"user_input" datastore:"user_input"`
//	SendMail           SyncData `json:"send_mail" datastore:"send_mail"`
//	SendSms            SyncData `json:"send_sms" datastore:"send_sms"`
//	Updates            SyncData `json:"updates" datastore:"updates"`
//	Notifications      SyncData `json:"notifications" datastore:"notifications"`
//	EmailTrigger       SyncData `json:"email_trigger" datastore:"email_trigger"`
//	AppExecutions      SyncData `json:"app_executions" datastore:"app_executions"`
//	WorkflowExecutions SyncData `json:"workflow_executions" datastore:"workflow_executions"`
//	Apps               SyncData `json:"apps" datastore:"apps"`
//	Workflows          SyncData `json:"workflows" datastore:"workflows"`
//	Autocomplete       SyncData `json:"autocomplete" datastore:"autocomplete"`
//	Authentication     SyncData `json:"authentication" datastore:"authentication"`
//	Schedule           SyncData `json:"schedule" datastore:"schedule"`
//}
//
//type SyncData struct {
//	Active         bool   `json:"active" datastore:"active"`
//	Type           string `json:"type,omitempty" datastore:"type"`
//	Name           string `json:"name,omitempty" datastore:"name"`
//	Description    string `json:"description,omitempty" datastore:"description"`
//	Limit          int64  `json:"limit,omitempty" datastore:"limit"`
//	StartDate      int64  `json:"start_date,omitempty" datastore:"start_date"`
//	EndDate        int64  `json:"end_date,omitempty" datastore:"end_date"`
//	DataCollection int64  `json:"data_collection,omitempty" datastore:"data_collection"`
//}
//
//type SyncConfig struct {
//	Interval int64  `json:"interval" datastore:"interval"`
//	Apikey   string `json:"api_key" datastore:"api_key"`
//}
//
//type PaymentSubscription struct {
//	Active           bool   `json:"active" datastore:"active"`
//	Startdate        int64  `json:"startdate" datastore:"startdate"`
//	CancellationDate int64  `json:"cancellationdate" datastore:"cancellationdate"`
//	Enddate          int64  `json:"enddate" datastore:"enddate"`
//	Name             string `json:"name" datastore:"name"`
//	Recurrence       string `json:"recurrence" datastore:"recurrence"`
//	Reference        string `json:"reference" datastore:"reference"`
//	Level            string `json:"level" datastore:"level"`
//	Amount           string `json:"amount" datastore:"amount"`
//	Currency         string `json:"currency" datastore:"currency"`
//}
//
//type Defaults struct {
//	AppDownloadRepo        string `json:"app_download_repo" datastore:"app_download_repo"`
//	AppDownloadBranch      string `json:"app_download_branch" datastore:"app_download_branch"`
//	WorkflowDownloadRepo   string `json:"workflow_download_repo" datastore:"workflow_download_repo"`
//	WorkflowDownloadBranch string `json:"workflow_download_branch" datastore:"workflow_download_branch"`
//}
//
//type AppAuthenticationStorage struct {
//	Active        bool                  `json:"active" datastore:"active"`
//	Label         string                `json:"label" datastore:"label"`
//	Id            string                `json:"id" datastore:"id"`
//	App           WorkflowApp           `json:"app" datastore:"app,noindex"`
//	Fields        []AuthenticationStore `json:"fields" datastore:"fields"`
//	Usage         []AuthenticationUsage `json:"usage" datastore:"usage"`
//	WorkflowCount int64                 `json:"workflow_count" datastore:"workflow_count"`
//	NodeCount     int64                 `json:"node_count" datastore:"node_count"`
//	OrgId         string                `json:"org_id" datastore:"org_id"`
//	Created       int64                 `json:"created" datastore:"created"`
//	Edited        int64                 `json:"edited" datastore:"edited"`
//	Defined       bool                  `json:"defined" datastore:"defined"`
//}
//
//type AuthenticationUsage struct {
//	WorkflowId string   `json:"workflow_id" datastore:"workflow_id"`
//	Nodes      []string `json:"nodes" datastore:"nodes"`
//}
//
//// An app inside Shuffle
//// Source      string `json:"source" datastore:"soure" yaml:"source"` - downloadlocation
//type WorkflowApp struct {
//	Name          string `json:"name" yaml:"name" required:true datastore:"name"`
//	IsValid       bool   `json:"is_valid" yaml:"is_valid" required:true datastore:"is_valid"`
//	ID            string `json:"id" yaml:"id,omitempty" required:false datastore:"id"`
//	Link          string `json:"link" yaml:"link" required:false datastore:"link,noindex"`
//	AppVersion    string `json:"app_version" yaml:"app_version" required:true datastore:"app_version"`
//	SharingConfig string `json:"sharing_config" yaml:"sharing_config" datastore:"sharing_config"`
//	Generated     bool   `json:"generated" yaml:"generated" required:false datastore:"generated"`
//	Downloaded    bool   `json:"downloaded" yaml:"downloaded" required:false datastore:"downloaded"`
//	Sharing       bool   `json:"sharing" yaml:"sharing" required:false datastore:"sharing"`
//	Verified      bool   `json:"verified" yaml:"verified" required:false datastore:"verified"`
//	Invalid       bool   `json:"invalid" yaml:"invalid" required:false datastore:"invalid"`
//	Activated     bool   `json:"activated" yaml:"activated" required:false datastore:"activated"`
//	Tested        bool   `json:"tested" yaml:"tested" required:false datastore:"tested"`
//	Owner         string `json:"owner" datastore:"owner" yaml:"owner"`
//	Hash          string `json:"hash" datastore:"hash" yaml:"hash"` // api.yaml+dockerfile+src/app.py for apps
//	PrivateID     string `json:"private_id" yaml:"private_id" required:false datastore:"private_id"`
//	Description   string `json:"description" datastore:"description,noindex" required:false yaml:"description"`
//	Environment   string `json:"environment" datastore:"environment" required:true yaml:"environment"`
//	SmallImage    string `json:"small_image" datastore:"small_image,noindex" required:false yaml:"small_image"`
//	LargeImage    string `json:"large_image" datastore:"large_image,noindex" yaml:"large_image" required:false`
//	ContactInfo   struct {
//		Name string `json:"name" datastore:"name" yaml:"name"`
//		Url  string `json:"url" datastore:"url" yaml:"url"`
//	} `json:"contact_info" datastore:"contact_info" yaml:"contact_info" required:false`
//	ReferenceInfo struct {
//		DocumentationUrl string `json:"documentation_url" datastore:"documentation_url"`
//		GithubUrl        string `json:"github_url" datastore:"github_url"`
//	}
//	FolderMount struct {
//		FolderMount       bool   `json:"folder_mount" datastore:"folder_mount"`
//		SourceFolder      string `json:"source_folder" datastore:"source_folder"`
//		DestinationFolder string `json:"destination_folder" datastore:"destination_folder"`
//	}
//	Actions        []WorkflowAppAction `json:"actions" yaml:"actions" required:true datastore:"actions,noindex"`
//	Authentication Authentication      `json:"authentication" yaml:"authentication" required:false datastore:"authentication"`
//	Tags           []string            `json:"tags" yaml:"tags" required:false datastore:"activated"`
//	Categories     []string            `json:"categories" yaml:"categories" required:false datastore:"categories"`
//	Created        int64               `json:"created" datastore:"created"`
//	Edited         int64               `json:"edited" datastore:"edited"`
//	LastRuntime    int64               `json:"last_runtime" datastore:"last_runtime"`
//}
//
//type WorkflowAppActionParameter struct {
//	Description    string           `json:"description" datastore:"description,noindex" yaml:"description"`
//	ID             string           `json:"id" datastore:"id" yaml:"id,omitempty"`
//	Name           string           `json:"name" datastore:"name" yaml:"name"`
//	Example        string           `json:"example" datastore:"example,noindex" yaml:"example"`
//	Value          string           `json:"value" datastore:"value,noindex" yaml:"value,omitempty"`
//	Multiline      bool             `json:"multiline" datastore:"multiline" yaml:"multiline"`
//	Options        []string         `json:"options" datastore:"options" yaml:"options"`
//	ActionField    string           `json:"action_field" datastore:"action_field" yaml:"actionfield,omitempty"`
//	Variant        string           `json:"variant" datastore:"variant" yaml:"variant,omitempty"`
//	Required       bool             `json:"required" datastore:"required" yaml:"required"`
//	Configuration  bool             `json:"configuration" datastore:"configuration" yaml:"configuration"`
//	Tags           []string         `json:"tags" datastore:"tags" yaml:"tags"`
//	Schema         SchemaDefinition `json:"schema" datastore:"schema" yaml:"schema"`
//	SkipMulticheck bool             `json:"skip_multicheck" datastore:"skip_multicheck" yaml:"skip_multicheck"`
//	ValueReplace   []Valuereplace   `json:"value_replace" datastore:"value_replace,noindex" yaml:"value_replace,omitempty"`
//	UniqueToggled  bool             `json:"unique_toggled" datastore:"unique_toggled" yaml:"unique_toggled"`
//}
//
//type Valuereplace struct {
//	Key   string `json:"key" datastore:"key" yaml:"key"`
//	Value string `json:"value" datastore:"value" yaml:"value"`
//}
//
//type SchemaDefinition struct {
//	Type string `json:"type" datastore:"type"`
//}
//
//type WorkflowAppAction struct {
//	Description       string                       `json:"description" datastore:"description,noindex"`
//	ID                string                       `json:"id" datastore:"id" yaml:"id,omitempty"`
//	Name              string                       `json:"name" datastore:"name"`
//	Label             string                       `json:"label" datastore:"label"`
//	NodeType          string                       `json:"node_type" datastore:"node_type"`
//	Environment       string                       `json:"environment" datastore:"environment"`
//	Sharing           bool                         `json:"sharing" datastore:"sharing"`
//	PrivateID         string                       `json:"private_id" datastore:"private_id"`
//	AppID             string                       `json:"app_id" datastore:"app_id"`
//	Tags              []string                     `json:"tags" datastore:"tags" yaml:"tags"`
//	Authentication    []AuthenticationStore        `json:"authentication" datastore:"authentication,noindex" yaml:"authentication,omitempty"`
//	Tested            bool                         `json:"tested" datastore:"tested" yaml:"tested"`
//	Parameters        []WorkflowAppActionParameter `json:"parameters" datastore: "parameters"`
//	ExecutionVariable struct {
//		Description string `json:"description" datastore:"description,noindex"`
//		ID          string `json:"id" datastore:"id"`
//		Name        string `json:"name" datastore:"name"`
//		Value       string `json:"value" datastore:"value,noindex"`
//	} `json:"execution_variable" datastore:"execution_variables"`
//	Returns struct {
//		Description string           `json:"description" datastore:"returns" yaml:"description,omitempty"`
//		Example     string           `json:"example" datastore:"example,noindex" yaml:"example"`
//		ID          string           `json:"id" datastore:"id" yaml:"id,omitempty"`
//		Schema      SchemaDefinition `json:"schema" datastore:"schema" yaml:"schema"`
//	} `json:"returns" datastore:"returns"`
//	AuthenticationId string `json:"authentication_id" datastore:"authentication_id"`
//	Example          string `json:"example,noindex" datastore:"example" yaml:"example"`
//	AuthNotRequired  bool   `json:"auth_not_required" datastore:"auth_not_required" yaml:"auth_not_required"`
//}

// FIXME: Generate a callback authentication ID?
// FIXME: Add org check ..
//type WorkflowExecution struct {
//	Type               string         `json:"type" datastore:"type"`
//	Status             string         `json:"status" datastore:"status"`
//	Start              string         `json:"start" datastore:"start"`
//	ExecutionArgument  string         `json:"execution_argument" datastore:"execution_argument,noindex"`
//	ExecutionId        string         `json:"execution_id" datastore:"execution_id"`
//	ExecutionSource    string         `json:"execution_source" datastore:"execution_source"`
//	ExecutionParent    string         `json:"execution_parent" datastore:"execution_parent"`
//	ExecutionOrg       string         `json:"execution_org" datastore:"execution_org"`
//	WorkflowId         string         `json:"workflow_id" datastore:"workflow_id"`
//	LastNode           string         `json:"last_node" datastore:"last_node"`
//	Authorization      string         `json:"authorization" datastore:"authorization"`
//	Result             string         `json:"result" datastore:"result,noindex"`
//	StartedAt          int64          `json:"started_at" datastore:"started_at"`
//	CompletedAt        int64          `json:"completed_at" datastore:"completed_at"`
//	ProjectId          string         `json:"project_id" datastore:"project_id"`
//	Locations          []string       `json:"locations" datastore:"locations"`
//	Workflow           Workflow       `json:"workflow" datastore:"workflow,noindex"`
//	Results            []ActionResult `json:"results" datastore:"results,noindex"`
//	ExecutionVariables []struct {
//		Description string `json:"description" datastore:"description,noindex"`
//		ID          string `json:"id" datastore:"id"`
//		Name        string `json:"name" datastore:"name"`
//		Value       string `json:"value" datastore:"value,noindex"`
//	} `json:"execution_variables,omitempty" datastore:"execution_variables,omitempty"`
//	OrgId string `json:"org_id" datastore:"org_id"`
//}

// This is for the nodes in a workflow, NOT the app action itself.
//type Action struct {
//	AppName           string                       `json:"app_name" datastore:"app_name"`
//	AppVersion        string                       `json:"app_version" datastore:"app_version"`
//	AppID             string                       `json:"app_id" datastore:"app_id"`
//	Errors            []string                     `json:"errors" datastore:"errors"`
//	ID                string                       `json:"id" datastore:"id"`
//	IsValid           bool                         `json:"is_valid" datastore:"is_valid"`
//	IsStartNode       bool                         `json:"isStartNode,omitempty" datastore:"isStartNode"`
//	Sharing           bool                         `json:"sharing,omitempty" datastore:"sharing"`
//	PrivateID         string                       `json:"private_id,omitempty" datastore:"private_id"`
//	Label             string                       `json:"label,omitempty" datastore:"label"`
//	SmallImage        string                       `json:"small_image,omitempty" datastore:"small_image,noindex" required:false yaml:"small_image"`
//	LargeImage        string                       `json:"large_image,omitempty" datastore:"large_image,noindex" yaml:"large_image" required:false`
//	Environment       string                       `json:"environment,omitempty" datastore:"environment"`
//	Name              string                       `json:"name" datastore:"name"`
//	Parameters        []WorkflowAppActionParameter `json:"parameters" datastore: "parameters,noindex"`
//	ExecutionVariable struct {
//		Description string `json:"description,omitempty" datastore:"description,noindex"`
//		ID          string `json:"id,omitempty" datastore:"id"`
//		Name        string `json:"name,omitempty" datastore:"name"`
//		Value       string `json:"value,omitempty" datastore:"value,noindex"`
//	} `json:"execution_variable,omitempty" datastore:"execution_variable,omitempty"`
//	Position struct {
//		X float64 `json:"x,omitempty" datastore:"x"`
//		Y float64 `json:"y,omitempty" datastore:"y"`
//	} `json:"position,omitempty"`
//	Priority         int    `json:"priority,omitempty" datastore:"priority"`
//	AuthenticationId string `json:"authentication_id" datastore:"authentication_id"`
//	Example          string `json:"example,omitempty" datastore:"example,noindex"`
//	AuthNotRequired  bool   `json:"auth_not_required,omitempty" datastore:"auth_not_required" yaml:"auth_not_required"`
//	Category         string `json:"category" datastore:"category"`
//}
//
//// Added environment for location to execute
//type Trigger struct {
//	AppName         string                       `json:"app_name" datastore:"app_name"`
//	Description     string                       `json:"description" datastore:"description,noindex"`
//	LongDescription string                       `json:"long_description" datastore:"long_description"`
//	Status          string                       `json:"status" datastore:"status"`
//	AppVersion      string                       `json:"app_version" datastore:"app_version"`
//	Errors          []string                     `json:"errors" datastore:"errors"`
//	ID              string                       `json:"id" datastore:"id"`
//	IsValid         bool                         `json:"is_valid" datastore:"is_valid"`
//	IsStartNode     bool                         `json:"isStartNode" datastore:"isStartNode"`
//	Label           string                       `json:"label" datastore:"label"`
//	SmallImage      string                       `json:"small_image" datastore:"small_image,noindex" required:false yaml:"small_image"`
//	LargeImage      string                       `json:"large_image" datastore:"large_image,noindex" yaml:"large_image" required:false`
//	Environment     string                       `json:"environment" datastore:"environment"`
//	TriggerType     string                       `json:"trigger_type" datastore:"trigger_type"`
//	Name            string                       `json:"name" datastore:"name"`
//	Tags            []string                     `json:"tags" datastore:"tags" yaml:"tags"`
//	Parameters      []WorkflowAppActionParameter `json:"parameters" datastore: "parameters,noindex"`
//	Position        struct {
//		X float64 `json:"x" datastore:"x"`
//		Y float64 `json:"y" datastore:"y"`
//	} `json:"position"`
//	Priority int `json:"priority" datastore:"priority"`
//}
//
//type Branch struct {
//	DestinationID string      `json:"destination_id" datastore:"destination_id"`
//	ID            string      `json:"id" datastore:"id"`
//	SourceID      string      `json:"source_id" datastore:"source_id"`
//	Label         string      `json:"label" datastore:"label"`
//	HasError      bool        `json:"has_errors" datastore: "has_errors"`
//	Conditions    []Condition `json:"conditions" datastore: "conditions,noindex"`
//}
//
//// Same format for a lot of stuff
//type Condition struct {
//	Condition   WorkflowAppActionParameter `json:"condition" datastore:"condition"`
//	Source      WorkflowAppActionParameter `json:"source" datastore:"source"`
//	Destination WorkflowAppActionParameter `json:"destination" datastore:"destination"`
//}
//
//type Schedule struct {
//	Name              string `json:"name" datastore:"name"`
//	Frequency         string `json:"frequency" datastore:"frequency"`
//	ExecutionArgument string `json:"execution_argument" datastore:"execution_argument,noindex"`
//	Id                string `json:"id" datastore:"id"`
//	OrgId             string `json:"org_id" datastore:"org_id"`
//	Environment       string `json:"environment" datastore:"environment"`
//}

//type Workflow struct {
//	Actions       []Action   `json:"actions" datastore:"actions,noindex"`
//	Branches      []Branch   `json:"branches" datastore:"branches,noindex"`
//	Triggers      []Trigger  `json:"triggers" datastore:"triggers,noindex"`
//	Schedules     []Schedule `json:"schedules" datastore:"schedules,noindex"`
//	Configuration struct {
//		ExitOnError  bool `json:"exit_on_error" datastore:"exit_on_error"`
//		StartFromTop bool `json:"start_from_top" datastore:"start_from_top"`
//	} `json:"configuration,omitempty" datastore:"configuration"`
//	Created           int64    `json:"created" datastore:"created"`
//	Edited            int64    `json:"edited" datastore:"edited"`
//	LastRuntime       int64    `json:"last_runtime" datastore:"last_runtime"`
//	Errors            []string `json:"errors,omitempty" datastore:"errors"`
//	Tags              []string `json:"tags,omitempty" datastore:"tags"`
//	ID                string   `json:"id" datastore:"id"`
//	IsValid           bool     `json:"is_valid" datastore:"is_valid"`
//	Name              string   `json:"name" datastore:"name"`
//	Description       string   `json:"description" datastore:"description,noindex"`
//	Start             string   `json:"start" datastore:"start"`
//	Owner             string   `json:"owner" datastore:"owner"`
//	Sharing           string   `json:"sharing" datastore:"sharing"`
//	Org               []Org    `json:"org,omitempty" datastore:"org"`
//	ExecutingOrg      Org      `json:"execution_org,omitempty" datastore:"execution_org"`
//	OrgId             string   `json:"org_id,omitempty" datastore:"org_id"`
//	WorkflowVariables []struct {
//		Description string `json:"description" datastore:"description,noindex"`
//		ID          string `json:"id" datastore:"id"`
//		Name        string `json:"name" datastore:"name"`
//		Value       string `json:"value" datastore:"value,noindex"`
//	} `json:"workflow_variables" datastore:"workflow_variables"`
//	ExecutionVariables []struct {
//		Description string `json:"description" datastore:"description,noindex"`
//		ID          string `json:"id" datastore:"id"`
//		Name        string `json:"name" datastore:"name"`
//		Value       string `json:"value" datastore:"value,noindex"`
//	} `json:"execution_variables,omitempty" datastore:"execution_variables"`
//	ExecutionEnvironment string     `json:"execution_environment" datastore:"execution_environment"`
//	PreviouslySaved      bool       `json:"previously_saved" datastore:"first_save"`
//	Categories           Categories `json:"categories" datastore:"categories"`
//	ExampleArgument      string     `json:"example_argument" datastore:"example_argument,noindex"`
//}

//type Category struct {
//	Name        string `json:"name" datastore:"name"`
//	Description string `json:"description" datastore:"description"`
//	Count       int64  `json:"count" datastore:"count"`
//}
//
//type Categories struct {
//	SIEM          Category `json:"siem" datastore:"siem"`
//	Communication Category `json:"communication" datastore:"communication"`
//	Assets        Category `json:"assets" datastore:"assets"`
//	Cases         Category `json:"cases" datastore:"cases"`
//	Network       Category `json:"network" datastore:"network"`
//	Intel         Category `json:"intel" datastore:"intel"`
//	EDR           Category `json:"edr" datastore:"edr"`
//	Other         Category `json:"other" datastore:"other"`
//}

//type ActionResult struct {
//	Action        Action `json:"action" datastore:"action,noindex"`
//	ExecutionId   string `json:"execution_id" datastore:"execution_id"`
//	Authorization string `json:"authorization" datastore:"authorization"`
//	Result        string `json:"result" datastore:"result,noindex"`
//	StartedAt     int64  `json:"started_at" datastore:"started_at"`
//	CompletedAt   int64  `json:"completed_at" datastore:"completed_at"`
//	Status        string `json:"status" datastore:"status"`
//}
//
//type Authentication struct {
//	Required   bool                   `json:"required" datastore:"required" yaml:"required" `
//	Parameters []AuthenticationParams `json:"parameters" datastore:"parameters" yaml:"parameters"`
//}
//
//type AuthenticationParams struct {
//	Description string           `json:"description" datastore:"description,noindex" yaml:"description"`
//	ID          string           `json:"id" datastore:"id" yaml:"id"`
//	Name        string           `json:"name" datastore:"name" yaml:"name"`
//	Example     string           `json:"example" datastore:"example,noindex" yaml:"example"`
//	Value       string           `json:"value,omitempty" datastore:"value,noindex" yaml:"value"`
//	Multiline   bool             `json:"multiline" datastore:"multiline" yaml:"multiline"`
//	Required    bool             `json:"required" datastore:"required" yaml:"required"`
//	In          string           `json:"in" datastore:"in" yaml:"in"`
//	Schema      SchemaDefinition `json:"schema" datastore:"schema" yaml:"schema"`
//	Scheme      string           `json:"scheme" datastore:"scheme" yaml:"scheme"` // Deprecated
//}
//
//type AuthenticationStore struct {
//	Key   string `json:"key" datastore:"key"`
//	Value string `json:"value" datastore:"value,noindex"`
//}
//
//type ExecutionRequestWrapper struct {
//	Data []ExecutionRequest `json:"data"`
//}
//
//type AppExecutionExample struct {
//	AppName         string   `json:"app_name" datastore:"app_name"`
//	AppVersion      string   `json:"app_version" datastore:"app_version"`
//	AppAction       string   `json:"app_action" datastore:"app_action"`
//	AppId           string   `json:"app_id" datastore:"app_id"`
//	ExampleId       string   `json:"example_id" datastore:"example_id"`
//	SuccessExamples []string `json:"success_examples" datastore:"success_examples,noindex"`
//	FailureExamples []string `json:"failure_examples" datastore:"failure_examples,noindex"`
//}

// This might be... a bit off, but that's fine :)
// This might also be stupid, as we want timelines and such
// Anyway, these are super basic stupid stats.
func increaseStatisticsField(ctx context.Context, fieldname, id string, amount int64, orgId string) error {

	// 1. Get current stats
	// 2. Increase field(s)
	// 3. Put new stats
	statisticsId := "global_statistics"
	nameKey := fieldname
	key := datastore.NameKey(statisticsId, nameKey, nil)

	statisticsItem := StatisticsItem{}
	newData := StatisticsData{
		Timestamp: int64(time.Now().Unix()),
		Amount:    amount,
		Id:        id,
	}

	if err := dbclient.Get(ctx, key, &statisticsItem); err != nil {
		// Should init
		if strings.Contains(fmt.Sprintf("%s", err), "entity") {
			statisticsItem = StatisticsItem{
				Total:     amount,
				OrgId:     orgId,
				Fieldname: fieldname,
				Data: []StatisticsData{
					newData,
				},
			}

			if _, err := dbclient.Put(ctx, key, &statisticsItem); err != nil {
				log.Printf("Error setting base stats: %s", err)
				return err
			}

			return nil
		}
		//log.Printf("STATSERR: %s", err)

		return err
	}

	statisticsItem.Total += amount
	statisticsItem.Data = append(statisticsItem.Data, newData)

	// New struct, to not add body, author etc
	// FIXME - reintroduce
	//if _, err := dbclient.Put(ctx, key, &statisticsItem); err != nil {
	//	log.Printf("Error stats to %s: %s", fieldname, err)
	//	return err
	//}

	//log.Printf("Stats: %#v", statisticsItem)

	return nil
}

func setWorkflowQueue(ctx context.Context, executionRequest shuffle.ExecutionRequest, env string) error {
	orgKey := fmt.Sprintf("workflowqueue-%s", env)
	key := datastore.NameKey(orgKey, executionRequest.ExecutionId, nil)

	// New struct, to not add body, author etc
	if _, err := dbclient.Put(ctx, key, &executionRequest); err != nil {
		log.Printf("Error adding workflow queue: %s", err)
		return err
	}

	return nil
}

//
//func setWorkflowQueue(ctx context.Context, executionRequests ExecutionRequestWrapper, id string) error {
//	key := datastore.NameKey("workflowqueue", id, nil)
//
//	// New struct, to not add body, author etc
//	if _, err := dbclient.Put(ctx, key, &executionRequests); err != nil {
//		log.Printf("Error adding workflow queue: %s", err)
//		return err
//	}
//
//	return nil
//}

func getWorkflowQueue(ctx context.Context, id string) (shuffle.ExecutionRequestWrapper, error) {
	orgId := fmt.Sprintf("workflowqueue-%s", id)
	q := datastore.NewQuery(orgId).Limit(10)
	executions := []shuffle.ExecutionRequest{}
	_, err := dbclient.GetAll(ctx, q, &executions)
	if err != nil {
		return shuffle.ExecutionRequestWrapper{}, err
	}

	return shuffle.ExecutionRequestWrapper{Data: executions}, nil

	//key := datastore.NameKey("workflowqueue", id, nil)
	//executions := ExecutionRequestWrapper{}
	//if err := dbclient.Get(ctx, key, &workflows); err != nil {
	//	return ExecutionRequestWrapper{}, err
	//}

	//return workflows, nil
}

//func setWorkflowqueuetest(id string) {
//	data := ExecutionRequestWrapper{
//		Data: []ExecutionRequest{
//			ExecutionRequest{
//				ExecutionId:   "2349bf96-51ad-68d2-5ca6-75ef8f7ee814",
//				WorkflowId:    "8e344a2e-db51-448f-804c-eb959a32c139",
//				Authorization: "wut",
//			},
//		},
//	}
//
//	err := setWorkflowQueue(data, id)
//	if err != nil {
//		log.Printf("Fail: %s", err)
//	}
//}

// Frequency = cronjob OR minutes between execution
func createSchedule(ctx context.Context, scheduleId, workflowId, name, startNode, frequency, orgId string, body []byte) error {
	var err error
	testSplit := strings.Split(frequency, "*")
	cronJob := ""
	newfrequency := 0

	if len(testSplit) > 5 {
		cronJob = frequency
	} else {
		newfrequency, err = strconv.Atoi(frequency)
		if err != nil {
			log.Printf("Failed to parse time: %s", err)
			return err
		}

		//if int(newfrequency) < 60 {
		//	cronJob = fmt.Sprintf("*/%s * * * *")
		//} else if int(newfrequency) <
	}

	// Reverse. Can't handle CRON, only numbers
	if len(cronJob) > 0 {
		return errors.New("cronJob isn't formatted correctly")
	}

	if newfrequency < 1 {
		return errors.New("Frequency has to be more than 0")
	}

	//log.Printf("CRON: %s, body: %s", cronJob, string(body))

	// FIXME:
	// This may run multiple places if multiple servers,
	// but that's a future problem
	//log.Printf("BODY: %s", string(body))
	parsedArgument := strings.Replace(string(body), "\"", "\\\"", -1)
	bodyWrapper := fmt.Sprintf(`{"start": "%s", "execution_source": "schedule", "execution_argument": "%s"}`, startNode, parsedArgument)
	log.Printf("WRAPPER BODY: \n%s", bodyWrapper)
	job := func() {
		request := &http.Request{
			URL:    &url.URL{},
			Method: "POST",
			Body:   ioutil.NopCloser(strings.NewReader(bodyWrapper)),
		}

		_, _, err := handleExecution(workflowId, shuffle.Workflow{ExecutingOrg: shuffle.Org{Id: orgId}}, request)
		if err != nil {
			log.Printf("Failed to execute %s: %s", workflowId, err)
		}
	}

	log.Printf("Starting frequency: %d", newfrequency)
	jobret, err := newscheduler.Every(newfrequency).Seconds().NotImmediately().Run(job)
	if err != nil {
		log.Printf("Failed to schedule workflow: %s", err)
		return err
	}

	//scheduledJobs = append(scheduledJobs, jobret)
	scheduledJobs[scheduleId] = jobret

	// Doesn't need running/not running. If stopped, we just delete it.
	timeNow := int64(time.Now().Unix())
	schedule := ScheduleOld{
		Id:                   scheduleId,
		WorkflowId:           workflowId,
		StartNode:            startNode,
		Argument:             string(body),
		WrappedArgument:      bodyWrapper,
		Seconds:              newfrequency,
		CreationTime:         timeNow,
		LastModificationtime: timeNow,
		LastRuntime:          timeNow,
		Org:                  orgId,
		Environment:          "onprem",
	}

	err = setSchedule(ctx, schedule)
	if err != nil {
		log.Printf("Failed to set schedule: %s", err)
		return err
	}

	// FIXME - Create a real schedule based on cron:
	// 1. Parse the cron in a function to match this schedule
	// 2. Make main init check for schedules that aren't running

	return nil
}

func handleGetWorkflowqueueConfirm(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// FIXME: Add authentication?
	id := request.Header.Get("Org-Id")
	if len(id) == 0 {
		log.Printf("No Org-Id header set - confirm")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Specify the org-id header."}`)))
		return
	}

	//setWorkflowqueuetest(id)
	ctx := context.Background()
	executionRequests, err := getWorkflowQueue(ctx, id)
	if err != nil {
		log.Printf("(1) Failed reading body for workflowqueue: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Entity parsing error - confirm"}`)))
		return
	}

	if len(executionRequests.Data) == 0 {
		log.Printf("[INFO] No requests to handle from queue")
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "Nothing in queue"}`)))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println("Failed reading body for stream result queue")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	// Getting from the request
	//log.Println(string(body))
	var removeExecutionRequests shuffle.ExecutionRequestWrapper
	err = json.Unmarshal(body, &removeExecutionRequests)
	if err != nil {
		log.Printf("Failed executionrequest in queue unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	if len(removeExecutionRequests.Data) == 0 {
		log.Printf("No requests to fix remove from DB")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Some removal error"}`)))
		return
	}

	// remove items from DB
	parsedId := fmt.Sprintf("workflowqueue-%s", id)
	ids := []string{}
	for _, execution := range removeExecutionRequests.Data {
		ids = append(ids, execution.ExecutionId)
	}

	err = DeleteKeys(ctx, parsedId, ids)
	if err != nil {
		log.Printf("[ERROR] Failed deleting %d execution keys for org %s", len(ids), id)
	} else {
		//log.Printf("[INFO] Deleted %d keys from org %s", len(ids), parsedId)
	}

	//var newExecutionRequests ExecutionRequestWrapper
	//for _, execution := range executionRequests.Data {
	//	found := false
	//	for _, removeExecution := range removeExecutionRequests.Data {
	//		if removeExecution.ExecutionId == execution.ExecutionId && removeExecution.WorkflowId == execution.WorkflowId {
	//			found = true
	//			break
	//		}
	//	}

	//	if !found {
	//		newExecutionRequests.Data = append(newExecutionRequests.Data, execution)
	//	}
	//}

	// Push only the remaining to the DB (remove)
	//if len(executionRequests.Data) != len(newExecutionRequests.Data) {
	//	err := setWorkflowQueue(ctx, newExecutionRequests, id)
	//	if err != nil {
	//		log.Printf("Fail: %s", err)
	//	}
	//}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

// FIXME: Authenticate this one? Can org ID be auth enough?
// (especially since we have a default: shuffle)
func handleGetWorkflowqueue(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	id := request.Header.Get("Org-Id")
	if len(id) == 0 {
		log.Printf("No org-id header set")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Specify the org-id header."}`)))
		return
	}

	ctx := context.Background()
	executionRequests, err := getWorkflowQueue(ctx, id)
	if err != nil {
		// Skipping as this comes up over and over
		//log.Printf("(2) Failed reading body for workflowqueue: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	if len(executionRequests.Data) == 0 {
		executionRequests.Data = []shuffle.ExecutionRequest{}
	} else {
		log.Printf("[INFO] Executionrequests (%s): %d", id, len(executionRequests.Data))
	}

	newjson, err := json.Marshal(executionRequests)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking workflow execution"}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(newjson)
}

func handleGetStreamResults(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

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
		log.Printf("Failed ActionResult unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	ctx := context.Background()
	workflowExecution, err := shuffle.GetWorkflowExecution(ctx, actionResult.ExecutionId)
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

// Finds the child nodes of a node in execution and returns them
// Used if e.g. a node in a branch is exited, and all children have to be stopped
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

// Checks if data is sent from Worker >0.8.51, which sends a full execution
// instead of individial results
func validateNewWorkerExecution(body []byte) error {
	//type WorkflowExecution struct {
	//}

	ctx := context.Background()
	var execution shuffle.WorkflowExecution
	err := json.Unmarshal(body, &execution)
	if err != nil {
		log.Printf("[WARNING] Failed execution unmarshaling: %s", err)
		return err
	}

	baseExecution, err := shuffle.GetWorkflowExecution(ctx, execution.ExecutionId)
	if err != nil {
		log.Printf("[ERROR] Failed getting execution (workflowqueue) %s: %s", execution.ExecutionId, err)
		return err
	}

	if baseExecution.Authorization != execution.Authorization {
		return errors.New("Bad authorization when validating execution")
	}

	// used to validate if it's actually the right marshal
	if len(baseExecution.Workflow.Actions) != len(execution.Workflow.Actions) {
		return errors.New(fmt.Sprintf("Bad length of actions (probably normal app): %d", len(execution.Workflow.Actions)))
	}

	if len(baseExecution.Workflow.Triggers) != len(execution.Workflow.Triggers) {
		return errors.New(fmt.Sprintf("Bad length of trigger: %d (probably normal app)", len(execution.Workflow.Triggers)))
	}

	if baseExecution.Status != "WAITING" && baseExecution.Status != "EXECUTING" {
		return errors.New(fmt.Sprintf("Workflow is already finished or failed. Can't update"))
	}

	if execution.Status == "EXECUTING" {
		//log.Printf("[INFO] Inside executing.")
		extra := 0
		for _, trigger := range execution.Workflow.Triggers {
			//log.Printf("Appname trigger (0): %s", trigger.AppName)
			if trigger.AppName == "User Input" || trigger.AppName == "Shuffle Workflow" {
				extra += 1
			}
		}

		if len(execution.Workflow.Actions)+extra == len(execution.Results) {
			execution.Status = "FINISHED"
		}

		log.Printf("[INFO] BASEEXECUTION LENGTH: %d", len(baseExecution.Workflow.Actions)+extra)
	}

	// FIXME: Add extra here
	//executionLength := len(baseExecution.Workflow.Actions)
	//if executionLength != len(execution.Results) {
	//	return errors.New(fmt.Sprintf("Bad length of actions vs results: want: %d have: %d", executionLength, len(execution.Results)))
	//}

	//log.Printf("\n\nSHOULD SET BACKEND DATA FOR EXEC \n\n")
	err = shuffle.SetWorkflowExecution(ctx, execution, true)
	if err == nil {
		log.Printf("[INFO] Set workflowexecution based on new worker (>0.8.53) for execution %s. Actions: %d, Triggers: %d, Results: %d", execution.ExecutionId, len(execution.Workflow.Actions), len(execution.Workflow.Triggers), len(execution.Results))
		//log.Printf("[INFO] Successfully set the execution to wait.")
	} else {
		log.Printf("[WARNING] Failed to set the execution to wait.")
	}

	return nil
}

func handleWorkflowQueue(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Println("(3) Failed reading body for workflowqueue")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	//log.Printf("Actionresult unmarshal: %s", string(body))
	err = validateNewWorkerExecution(body)
	if err == nil {
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "Success"}`)))
		return
	} else {
		//log.Printf("[WARNING] Handling other execution variant: %s", err)
	}

	var actionResult shuffle.ActionResult
	err = json.Unmarshal(body, &actionResult)
	if err != nil {
		log.Printf("Failed ActionResult unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	// 1. Get the WorkflowExecution(ExecutionId) from the database
	// 2. if ActionResult.Authentication != WorkflowExecution.Authentication -> exit
	// 3. Add to and update actionResult in workflowExecution
	// 4. Push to db
	// IF FAIL: Set executionstatus: abort or cancel

	ctx := context.Background()
	workflowExecution, err := shuffle.GetWorkflowExecution(ctx, actionResult.ExecutionId)
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

	if actionResult.Status == "WAITING" && actionResult.Action.AppName == "User Input" {
		log.Printf("SHOULD WAIT A BIT AND RUN CLOUD STUFF WITH USER INPUT! WAITING!")

		var trigger shuffle.Trigger
		err = json.Unmarshal([]byte(actionResult.Result), &trigger)
		if err != nil {
			log.Printf("Failed unmarshaling actionresult for user input: %s", err)
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		orgId := workflowExecution.ExecutionOrg
		if len(workflowExecution.OrgId) == 0 && len(workflowExecution.Workflow.OrgId) > 0 {
			orgId = workflowExecution.Workflow.OrgId
		}

		err := handleUserInput(trigger, orgId, workflowExecution.Workflow.ID, workflowExecution.ExecutionId)
		if err != nil {
			log.Printf("Failed userinput handler: %s", err)
			actionResult.Result = fmt.Sprintf("Cloud error: %s", err)
			workflowExecution.Results = append(workflowExecution.Results, actionResult)
			workflowExecution.Status = "ABORTED"
			err = shuffle.SetWorkflowExecution(ctx, *workflowExecution, true)
			if err != nil {
				log.Printf("Failed to set execution during wait")
			} else {
				log.Printf("Successfully set the execution to waiting.")
			}

			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Error: %s"}`, err)))
		} else {
			log.Printf("Successful userinput handler")
			resp.WriteHeader(200)
			resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "CLOUD IS DONE"}`)))

			actionResult.Result = "Waiting for user feedback based on configuration"

			workflowExecution.Results = append(workflowExecution.Results, actionResult)
			workflowExecution.Status = actionResult.Status
			err = shuffle.SetWorkflowExecution(ctx, *workflowExecution, true)
			if err != nil {
				log.Printf("Failed ")
			} else {
				log.Printf("Successfully set the execution to waiting.")
			}
		}

		return
	}

	runWorkflowExecutionTransaction(ctx, 0, workflowExecution.ExecutionId, actionResult, resp)
}

// Will make sure transactions are always ran for an execution. This is recursive if it fails. Allowed to fail up to 5 times
func runWorkflowExecutionTransaction(ctx context.Context, attempts int64, workflowExecutionId string, actionResult shuffle.ActionResult, resp http.ResponseWriter) {
	// Should start a tx for the execution here
	workflowExecution, err := shuffle.GetWorkflowExecution(ctx, workflowExecutionId)
	if err != nil {
		log.Printf("[ERROR] Failed getting execution cache: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting execution"}`)))
		return
	}

	//resultLength := len(workflowExecution.Results)
	dbSave := false
	setExecution := true

	if actionResult.Action.ID == "" {
		//log.Printf("[ERROR] Failed handling EMPTY action %#v", actionResult)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Can't handle empty action"}`)))
		return
	}
	//tx, err := dbclient.NewTransaction(ctx)
	//if err != nil {
	//	log.Printf("client.NewTransaction: %v", err)
	//	resp.WriteHeader(401)
	//	resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed creating transaction"}`)))
	//	return
	//}

	//key := datastore.NameKey("workflowexecution", workflowExecutionId, nil)
	//workflowExecution := &WorkflowExecution{}
	//if err := tx.Get(key, workflowExecution); err != nil {
	//	log.Printf("[ERROR] tx.Get bug: %v", err)
	//	tx.Rollback()
	//	resp.WriteHeader(401)
	//	resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting the workflow key"}`)))
	//	return
	//}

	if actionResult.Status == "ABORTED" || actionResult.Status == "FAILURE" {
		dbSave = true

		newResults := []shuffle.ActionResult{}
		childNodes := []string{}
		if workflowExecution.Workflow.Configuration.ExitOnError {
			log.Printf("[WARNING] Actionresult is %s for node %s in %s. Should set workflowExecution and exit all running functions", actionResult.Status, actionResult.Action.ID, workflowExecution.ExecutionId)
			workflowExecution.Status = actionResult.Status
			workflowExecution.LastNode = actionResult.Action.ID
			// Find underlying nodes and add them
		} else {
			log.Printf("[WARNING] Actionresult is %s for node %s in %s. Continuing anyway because of workflow configuration.", actionResult.Status, actionResult.Action.ID, workflowExecution.ExecutionId)

			// Finds ALL childnodes to set them to SKIPPED
			childNodes = findChildNodes(*workflowExecution, actionResult.Action.ID)

			// Remove duplicates
			//log.Printf("CHILD NODES: %d", len(childNodes))
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
								log.Printf("Not setting node %s to SKIPPED", nodeId)
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

						newResults = append(newResults, newResult)
						//increaseStatisticsField(ctx, "workflow_execution_actions_skipped", workflowExecution.Workflow.ID, 1, workflowExecution.ExecutionOrg)
					}
				}
			}
		}

		// Cleans up aborted, and always gives a result
		lastResult := ""
		// type ActionResult struct {
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

		if workflowExecution.Status == "ABORTED" {
			//err = increaseStatisticsField(ctx, "workflow_executions_aborted", workflowExecution.Workflow.ID, 1, workflowExecution.ExecutionOrg)
			//if err != nil {
			//	log.Printf("Failed to increase aborted execution stats: %s", err)
			//}
		} else if workflowExecution.Status == "FAILURE" {
			//err = increaseStatisticsField(ctx, "workflow_executions_failure", workflowExecution.Workflow.ID, 1, workflowExecution.ExecutionOrg)
			//if err != nil {
			//	log.Printf("Failed to increase failure execution stats: %s", err)
			//}
		}
	}

	// FIXME rebuild to be like this or something
	// workflowExecution/ExecutionId/Nodes/NodeId
	// Find the appropriate action
	//log.Printf("[INFO] Setting value of %s in workflow %s to %s (1)", actionResult.Action.ID, workflowExecution.ExecutionId, actionResult.Status)
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
			actionVarName := workflowExecution.Results[outerindex].Action.ExecutionVariable.Name
			// Finds potential execution arguments
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

			log.Printf("[INFO] Updating %s in workflow %s from %s to %s (3)", actionResult.Action.ID, workflowExecution.ExecutionId, workflowExecution.Results[outerindex].Status, actionResult.Status)
			workflowExecution.Results[outerindex] = actionResult
		} else {
			log.Printf("[INFO] Setting value of %s in workflow %s to %s (1)", actionResult.Action.ID, workflowExecution.ExecutionId, actionResult.Status)
			workflowExecution.Results = append(workflowExecution.Results, actionResult)
		}
	} else {
		log.Printf("[INFO] Setting value of %s in workflow %s to %s (2)", actionResult.Action.ID, workflowExecution.ExecutionId, actionResult.Status)
		workflowExecution.Results = append(workflowExecution.Results, actionResult)
	}

	// FIXME: Have a check for skippednodes and their parents
	for resultIndex, result := range workflowExecution.Results {
		if result.Status != "SKIPPED" {
			continue
		}

		// Checks if all parents are skipped or failed. Otherwise removes them from the results
		for _, branch := range workflowExecution.Workflow.Branches {
			if branch.DestinationID == result.Action.ID {
				for _, subresult := range workflowExecution.Results {
					if subresult.Action.ID == branch.SourceID {
						if subresult.Status != "SKIPPED" && subresult.Status != "FAILURE" {
							log.Printf("SUBRESULT PARENT STATUS: %s", subresult.Status)
							log.Printf("Should remove resultIndex: %d", resultIndex)

							workflowExecution.Results = append(workflowExecution.Results[:resultIndex], workflowExecution.Results[resultIndex+1:]...)

							break
						}
					}
				}
			}
		}
	}

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

			//err = increaseStatisticsField(ctx, "workflow_executions_success", workflowExecution.Workflow.ID, 1, workflowExecution.ExecutionOrg)
			//if err != nil {
			//	log.Printf("Failed to increase success execution stats: %s", err)
			//}

			// Handles extra statistics stuff when it's done
			// Does autocomplete magic with JSON
			handleExecutionStatistics(*workflowExecution)
		}
	}

	// FIXME - why isn't this how it works otherwise, wtf?
	//workflow, err := shuffle.GetWorkflow(workflowExecution.Workflow.ID)
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
	cache, err := shuffle.GetCache(ctx, cacheKey)
	if err == nil {
		cacheData := []byte(cache.([]uint8))
		//log.Printf("CACHEDATA: %#v", cacheData)
		err = json.Unmarshal(cacheData, &workflowExecution)
		if err == nil {
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

	//if value, found := requestCache.Get(cacheKey); found {
	//	parsedValue := value.(*shuffle.WorkflowExecution)
	//	if len(parsedValue.Results) > 0 && len(parsedValue.Results) != resultLength {
	//		setExecution = false
	//		if attempts > 5 {
	//			//log.Printf("\n\nSkipping execution input - %d vs %d. Attempts: (%d)\n\n", len(parsedValue.Results), resultLength, attempts)
	//		}

	//		attempts += 1
	//		if len(workflowExecution.Results) <= len(workflowExecution.Workflow.Actions) {
	//			runWorkflowExecutionTransaction(ctx, attempts, workflowExecutionId, actionResult, resp)
	//			return
	//		}
	//	}
	//}

	if setExecution || workflowExecution.Status == "FINISHED" || workflowExecution.Status == "ABORTED" || workflowExecution.Status == "FAILURE" {
		err = shuffle.SetWorkflowExecution(ctx, *workflowExecution, dbSave)
		if err != nil {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed setting workflowexecution actionresult: %s"}`, err)))
			return
		}
	} else {
		log.Printf("Skipping setexec with status %s", workflowExecution.Status)
	}

	//ExecutionId
	// Transactions: https://cloud.google.com/datastore/docs/concepts/transactions#datastore-datastore-transactional-update-go
	// Prevents timing issues
	//if _, err := tx.Put(key, workflowExecution); err != nil {
	//	log.Printf("[ERROR] tx.Put error: %v", err)
	//	err = tx.Rollback()
	//	if err != nil {
	//		log.Printf("[ERROR] Rollback error (3): %s", err)
	//	}

	//	resp.WriteHeader(401)
	//	resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed setting workflowexecution actionresult: %s"}`, err)))
	//	return
	//}

	//if _, err = tx.Commit(); err != nil {
	//	err = tx.Rollback()
	//	if err != nil {
	//		log.Printf("[ERROR] Rollback error expected ? (1): %s", err)
	//	}

	//	if attempts >= 7 {
	//		log.Printf("[ERROR] QUITTING: tx.Commit %d: %v", attempts, err)

	//		workflowExecution.Status = "ABORTED"
	//		shuffle.SetWorkflowExecution(ctx, *workflowExecution, true)

	//		resp.WriteHeader(401)
	//		resp.Write([]byte(`{"success": false}`))
	//		return
	//	}

	//	if attempts > 3 {
	//		log.Printf("[WARNING] tx.Commit %d: %v", attempts, err)
	//	}

	//	attempts += 1
	//	runWorkflowExecutionTransaction(ctx, attempts, workflowExecutionId, actionResult, resp)
	//	return
	//} else {
	//	//if grpc.Code(err) == codes.Aborted {
	//	//	return nil, ErrConcurrentTransaction
	//	//}
	//	//t.id = nil // mark the transaction as expired
	//}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

func JSONCheck(str string) bool {
	var jsonStr interface{}
	return json.Unmarshal([]byte(str), &jsonStr) == nil
}

func handleExecutionStatistics(execution shuffle.WorkflowExecution) {
	// FIXME: CLEAN UP THE JSON THAT'S SAVED.
	// https://github.com/frikky/Shuffle/issues/172
	appResults := []shuffle.AppExecutionExample{}
	for _, result := range execution.Results {
		resultCheck := JSONCheck(result.Result)
		if !resultCheck {
			//log.Printf("Result is NOT JSON!")
			continue
		} else {
			//log.Printf("Result IS JSON!")

		}

		appFound := false
		executionIndex := 0
		for index, appExample := range appResults {
			if appExample.AppId == result.Action.ID {
				appFound = true
				executionIndex = index
				break
			}
		}

		if appFound {
			// Append to SuccessExamples or FailureExamples
			if result.Status == "ABORTED" || result.Status == "FAILURE" {
				appResults[executionIndex].FailureExamples = append(appResults[executionIndex].FailureExamples, result.Result)
			} else if result.Status == "FINISHED" || result.Status == "SUCCESS" {
				appResults[executionIndex].SuccessExamples = append(appResults[executionIndex].SuccessExamples, result.Result)
			} else {
				log.Printf("[ERROR] Can't handle status %s", result.Status)
			}

			// appResults = append(appResults, executionExample)

		} else {
			// CREATE SuccessExamples or FailureExamples
			executionExample := shuffle.AppExecutionExample{
				AppName:    result.Action.AppName,
				AppVersion: result.Action.AppVersion,
				AppAction:  result.Action.Name,
				AppId:      result.Action.AppID,
				ExampleId:  fmt.Sprintf("%s_%s", execution.ExecutionId, result.Action.AppID),
			}

			if result.Status == "ABORTED" || result.Status == "FAILURE" {
				executionExample.FailureExamples = append(executionExample.FailureExamples, result.Result)
			} else if result.Status == "FINISHED" || result.Status == "SUCCESS" {
				executionExample.SuccessExamples = append(executionExample.SuccessExamples, result.Result)
			} else {
				log.Printf("[ERROR] Can't handle status %s", result.Status)
			}

			appResults = append(appResults, executionExample)
		}
	}

	// ExampleId string `json:"example_id"`
	// func setExampleresult(ctx context.Context, result exampleResult) error {
	// log.Printf("Execution length: %d", len(appResults))
	if len(appResults) > 0 {
		ctx := context.Background()
		successful := 0
		for _, exampleresult := range appResults {
			err := setExampleresult(ctx, exampleresult)
			if err != nil {
				log.Printf("[ERROR] Failed setting examplresult %s: %s", exampleresult.ExampleId, err)
			} else {
				successful += 1
			}
		}

		log.Printf("[INFO] Added %d exampleresults to backend", successful)
	} else {
		//log.Printf("[INFO] No example results necessary to be added for execution %s", execution.ExecutionId)
	}
}

func getWorkflows(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in getworkflows: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	//memcacheName := fmt.Sprintf("%s_workflows", user.Username)
	ctx := context.Background()
	//if item, err := memcache.Get(ctx, memcacheName); err == memcache.ErrCacheMiss {
	//	// Not in cache
	//	//log.Printf("Workflows not in cache.")
	//} else if err != nil {
	//	log.Printf("Error getting item: %v", err)
	//} else {
	//	// FIXME - verify if value is ok? Can unmarshal etc.
	//	resp.WriteHeader(200)
	//	resp.Write(item.Value)
	//	return
	//}

	// With user, do a search for workflows with user or user's org attached
	q := datastore.NewQuery("workflow").Filter("owner =", user.Id)
	if user.Role == "admin" {
		q = datastore.NewQuery("workflow").Filter("org_id =", user.ActiveOrg.Id)
		log.Printf("[INFO] Getting workflows (ADMIN) for organization %s", user.ActiveOrg.Id)
	}

	q = q.Order("-edited")

	var workflows []shuffle.Workflow
	_, err = dbclient.GetAll(ctx, q, &workflows)
	if err != nil {
		if strings.Contains(fmt.Sprintf("%s", err), "ResourceExhausted") {
			q = q.Limit(35)
			_, err = dbclient.GetAll(ctx, q, &workflows)
			if err != nil {
				log.Printf("Failed getting workflows for user %s: %s (0)", user.Username, err)
				resp.WriteHeader(401)
				resp.Write([]byte(`{"success": false}`))
				return
			}
		} else {
			log.Printf("Failed getting workflows for user %s: %s (1)", user.Username, err)
			//DeleteKey(ctx, "workflow", "5694357e-8063-4580-8529-301cc72df951")

			//log.Printf("Workflows: %#v", workflows)
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}
	}

	if len(workflows) == 0 {
		resp.WriteHeader(200)
		resp.Write([]byte("[]"))
		return
	}

	newjson, err := json.Marshal(workflows)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking workflows"}`)))
		return
	}

	//item := &memcache.Item{
	//	Key:        memcacheName,
	//	Value:      newjson,
	//	Expiration: time.Minute * 10,
	//}
	//if err := memcache.Add(ctx, item); err == memcache.ErrNotStored {
	//	if err := memcache.Set(ctx, item); err != nil {
	//		log.Printf("Error setting item: %v", err)
	//	}
	//} else if err != nil {
	//	log.Printf("Error adding item: %v", err)
	//} else {
	//	//log.Printf("Set cache for %s", item.Key)
	//}

	resp.WriteHeader(200)
	resp.Write(newjson)
}

func deleteWorkflow(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in deleting workflow: %s", err)
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

	if len(fileId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID to delete is not valid"}`))
		return
	}

	ctx := context.Background()
	workflow, err := shuffle.GetWorkflow(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting the workflow locally (delete workflow): %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - have a check for org etc too..
	if user.Id != workflow.Owner {
		log.Printf("Wrong user (%s) for workflow %s", user.Username, workflow.ID)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Clean up triggers and executions
	for _, item := range workflow.Triggers {
		if item.TriggerType == "SCHEDULE" && item.Status != "uninitialized" {
			err = deleteSchedule(ctx, item.ID)
			if err != nil {
				log.Printf("Failed to delete schedule: %s - is it started?", err)
			}
		} else if item.TriggerType == "WEBHOOK" {
			//err = removeWebhookFunction(ctx, item.ID)
			//if err != nil {
			//	log.Printf("Failed to delete webhook: %s", err)
			//}
		} else if item.TriggerType == "EMAIL" {
			err = handleOutlookSubRemoval(ctx, user, workflow.ID, item.ID)
			if err != nil {
				log.Printf("Failed to delete email sub: %s", err)
			}
		}

		err = increaseStatisticsField(ctx, "total_workflow_triggers", workflow.ID, -1, workflow.OrgId)
		if err != nil {
			log.Printf("Failed to increase total workflows: %s", err)
		}
	}

	// FIXME - maybe delete workflow executions
	log.Printf("[INFO] Should have deleted workflow %s", fileId)
	err = DeleteKey(ctx, "workflow", fileId)
	if err != nil {
		log.Printf("Failed deleting key %s", fileId)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Failed deleting key"}`))
		return
	}

	//err = increaseStatisticsField(ctx, "total_workflows", fileId, -1, workflow.OrgId)
	//if err != nil {
	//	log.Printf("Failed to increase total workflows: %s", err)
	//}
	//memcacheName := fmt.Sprintf("%s_%s", user.Username, fileId)
	//memcache.Delete(ctx, memcacheName)
	//memcacheName = fmt.Sprintf("%s_workflows", user.Username)
	//memcache.Delete(ctx, memcacheName)

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

// Identifies what a category defined really is

func getWorkflowLocal(fileId string, request *http.Request) ([]byte, error) {
	fullUrl := fmt.Sprintf("%s/api/v1/workflows/%s", localBase, fileId)
	client := &http.Client{}
	req, err := http.NewRequest(
		"GET",
		fullUrl,
		nil,
	)

	if err != nil {
		return []byte{}, err
	}

	for key, value := range request.Header {
		req.Header.Add(key, strings.Join(value, ";"))
	}

	newresp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}

	body, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		return []byte{}, err
	}

	// Temporary solution
	if strings.Contains(string(body), "reason") && strings.Contains(string(body), "false") {
		return []byte{}, errors.New(fmt.Sprintf("Failed getting workflow %s with message %s", fileId, string(body)))
	}

	return body, nil
}

//// New execution with firestore

func cleanupExecutions(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("[INFO] Api authentication failed in cleanup executions: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "Not authenticated"}`))
		return
	}

	if user.Role != "admin" {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "message": "Insufficient permissions"}`))
		return
	}

	ctx := context.Background()

	// Removes three months from today
	timestamp := int64(time.Now().AddDate(0, -2, 0).Unix())
	log.Println(timestamp)
	q := datastore.NewQuery("workflowexecution").Filter("started_at <", timestamp)
	var workflowExecutions []shuffle.WorkflowExecution
	_, err = dbclient.GetAll(ctx, q, &workflowExecutions)
	if err != nil {
		log.Printf("Error getting workflowexec (cleanup): %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting all workflowexecutions"}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

func handleExecution(id string, workflow shuffle.Workflow, request *http.Request) (shuffle.WorkflowExecution, string, error) {
	ctx := context.Background()
	if workflow.ID == "" || workflow.ID != id {
		tmpworkflow, err := shuffle.GetWorkflow(ctx, id)
		if err != nil {
			log.Printf("Failed getting the workflow locally (execution cleanup): %s", err)
			return shuffle.WorkflowExecution{}, "Failed getting workflow", err
		}

		workflow = *tmpworkflow
	}

	if len(workflow.ExecutingOrg.Id) == 0 {
		log.Printf("[INFO] Stopped execution because there is no executing org for workflow %s", workflow.ID)
		return shuffle.WorkflowExecution{}, fmt.Sprintf("Workflow has no executing org defined"), errors.New("Workflow has no executing org defined")
	}

	if len(workflow.Actions) == 0 {
		workflow.Actions = []shuffle.Action{}
	} else {
		newactions := []shuffle.Action{}
		for _, action := range workflow.Actions {
			action.LargeImage = ""
			action.SmallImage = ""
			newactions = append(newactions, action)
			//log.Printf("ACTION: %#v", action)
		}

		workflow.Actions = newactions
	}

	if len(workflow.Branches) == 0 {
		workflow.Branches = []shuffle.Branch{}
	}
	if len(workflow.Triggers) == 0 {
		workflow.Triggers = []shuffle.Trigger{}
	} else {
		newtriggers := []shuffle.Trigger{}
		for _, trigger := range workflow.Triggers {
			trigger.LargeImage = ""
			trigger.SmallImage = ""
			newtriggers = append(newtriggers, trigger)
			//log.Printf("ACTION: %#v", trigger)
		}

		workflow.Triggers = newtriggers
	}

	if len(workflow.Errors) == 0 {
		workflow.Errors = []string{}
	}

	if !workflow.IsValid {
		log.Printf("[ERROR] Stopped execution as workflow %s is not valid.", workflow.ID)
		return shuffle.WorkflowExecution{}, fmt.Sprintf(`workflow %s is invalid`, workflow.ID), errors.New("Failed getting workflow")
	}

	workflowBytes, err := json.Marshal(workflow)
	if err != nil {
		log.Printf("Failed workflow unmarshal in execution: %s", err)
		return shuffle.WorkflowExecution{}, "", err
	}

	//log.Println(workflow)
	var workflowExecution shuffle.WorkflowExecution
	err = json.Unmarshal(workflowBytes, &workflowExecution.Workflow)
	if err != nil {
		log.Printf("Failed execution unmarshaling: %s", err)
		return shuffle.WorkflowExecution{}, "Failed unmarshal during execution", err
	}

	makeNew := true
	start, startok := request.URL.Query()["start"]
	if request.Method == "POST" {
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			log.Printf("[ERROR] Failed request POST read: %s", err)
			return shuffle.WorkflowExecution{}, "Failed getting body", err
		}

		// This one doesn't really matter.
		log.Printf("[INFO] Running POST execution with body of length %d for workflow %s", len(string(body)), workflowExecution.Workflow.ID)

		if len(body) >= 4 {
			if body[0] == 34 && body[len(body)-1] == 34 {
				body = body[1 : len(body)-1]
			}
			if body[0] == 34 && body[len(body)-1] == 34 {
				body = body[1 : len(body)-1]
			}
		}

		//workflowExecution.ExecutionSource = "default"
		sourceWorkflow, sourceWorkflowOk := request.URL.Query()["source_workflow"]
		if sourceWorkflowOk {
			//log.Printf("Got source workflow %s", sourceWorkflow)
			workflowExecution.ExecutionSource = sourceWorkflow[0]
		} else {
			//log.Printf("Did NOT get source workflow")

		}

		sourceExecution, sourceExecutionOk := request.URL.Query()["source_execution"]
		if sourceExecutionOk {
			//log.Printf("[INFO] Got source execution%s", sourceExecution)
			workflowExecution.ExecutionParent = sourceExecution[0]
		} else {
			//log.Printf("Did NOT get source execution")
		}

		if len(string(body)) < 50 {
			//log.Println(body)
			// String in string
			//log.Println(body)

			//if string(body)[0] == "\"" && string(body)[string(body)
			log.Printf("Body: %s", string(body))
		}

		var execution shuffle.ExecutionRequest
		err = json.Unmarshal(body, &execution)
		if err != nil {
			log.Printf("[WARNING] Failed execution POST unmarshaling - continuing anyway: %s", err)
			//return shuffle.WorkflowExecution{}, "", err
		}

		if execution.Start == "" && len(body) > 0 {
			execution.ExecutionArgument = string(body)
		}

		// FIXME - this should have "execution_argument" from executeWorkflow frontend
		//log.Printf("EXEC: %#v", execution)
		if len(execution.ExecutionArgument) > 0 {
			workflowExecution.ExecutionArgument = execution.ExecutionArgument
		}

		if len(execution.ExecutionSource) > 0 {
			workflowExecution.ExecutionSource = execution.ExecutionSource
		}

		//log.Printf("Execution data: %#v", execution)
		if len(execution.Start) == 36 {
			log.Printf("[INFO] Should start execution on node %s", execution.Start)
			workflowExecution.Start = execution.Start

			found := false
			for _, action := range workflow.Actions {
				if action.ID == workflow.Start {
					found = true
				}
			}

			if !found {
				log.Printf("[ERROR] ACTION %s WAS NOT FOUND!", workflow.Start)
				return shuffle.WorkflowExecution{}, fmt.Sprintf("Startnode %s was not found in actions", workflow.Start), errors.New(fmt.Sprintf("Startnode %s was not found in actions", workflow.Start))
			}
		} else if len(execution.Start) > 0 {

			log.Printf("[ERROR] START ACTION %s IS WRONG ID LENGTH %d!", execution.Start, len(execution.Start))
			return shuffle.WorkflowExecution{}, fmt.Sprintf("Startnode %s was not found in actions", execution.Start), errors.New(fmt.Sprintf("Startnode %s was not found in actions", execution.Start))
		}

		if len(execution.ExecutionId) == 36 {
			workflowExecution.ExecutionId = execution.ExecutionId
		} else {
			sessionToken := uuid.NewV4()
			workflowExecution.ExecutionId = sessionToken.String()
		}
	} else {
		// Check for parameters of start and ExecutionId
		// This is mostly used for user input trigger

		answer, answerok := request.URL.Query()["answer"]
		referenceId, referenceok := request.URL.Query()["reference_execution"]
		if answerok && referenceok {
			// If answer is false, reference execution with result
			log.Printf("[INFO] Answer is OK AND reference is OK!")
			if answer[0] == "false" {
				log.Printf("Should update reference and return, no need for further execution!")

				// Get the reference execution
				oldExecution, err := shuffle.GetWorkflowExecution(ctx, referenceId[0])
				if err != nil {
					log.Printf("Failed getting execution (execution) %s: %s", referenceId[0], err)
					return shuffle.WorkflowExecution{}, fmt.Sprintf("Failed getting execution ID %s because it doesn't exist.", referenceId[0]), err
				}

				if oldExecution.Workflow.ID != id {
					log.Println("Wrong workflowid!")
					return shuffle.WorkflowExecution{}, fmt.Sprintf("Bad ID %s", referenceId), errors.New("Bad ID")
				}

				newResults := []shuffle.ActionResult{}
				//log.Printf("%#v", oldExecution.Results)
				for _, result := range oldExecution.Results {
					log.Printf("%s - %s", result.Action.ID, start[0])
					if result.Action.ID == start[0] {
						note, noteok := request.URL.Query()["note"]
						if noteok {
							result.Result = fmt.Sprintf("User note: %s", note[0])
						} else {
							result.Result = fmt.Sprintf("User clicked %s", answer[0])
						}

						// Stopping the whole thing
						result.CompletedAt = int64(time.Now().Unix())
						result.Status = "ABORTED"
						oldExecution.Status = result.Status
						oldExecution.Result = result.Result
						oldExecution.LastNode = result.Action.ID
					}

					newResults = append(newResults, result)
				}

				oldExecution.Results = newResults
				err = shuffle.SetWorkflowExecution(ctx, *oldExecution, true)
				if err != nil {
					log.Printf("Error saving workflow execution actionresult setting: %s", err)
					return shuffle.WorkflowExecution{}, fmt.Sprintf("Failed setting workflowexecution actionresult in execution: %s", err), err
				}

				return shuffle.WorkflowExecution{}, "", nil
			}
		}

		if referenceok {
			log.Printf("Handling an old execution continuation!")
			// Will use the old name, but still continue with NEW ID
			oldExecution, err := shuffle.GetWorkflowExecution(ctx, referenceId[0])
			if err != nil {
				log.Printf("Failed getting execution (execution) %s: %s", referenceId[0], err)
				return shuffle.WorkflowExecution{}, fmt.Sprintf("Failed getting execution ID %s because it doesn't exist.", referenceId[0]), err
			}

			workflowExecution = *oldExecution
		}

		if len(workflowExecution.ExecutionId) == 0 {
			sessionToken := uuid.NewV4()
			workflowExecution.ExecutionId = sessionToken.String()
		} else {
			log.Printf("Using the same executionId as before: %s", workflowExecution.ExecutionId)
			makeNew = false
		}

		// Don't override workflow defaults
	}

	if startok {
		//log.Printf("\n\n[INFO] Setting start to %s based on query!\n\n", start[0])
		//workflowExecution.Workflow.Start = start[0]
		workflowExecution.Start = start[0]
	}

	// FIXME - regex uuid, and check if already exists?
	if len(workflowExecution.ExecutionId) != 36 {
		log.Printf("Invalid uuid: %s", workflowExecution.ExecutionId)
		return shuffle.WorkflowExecution{}, "Invalid uuid", err
	}

	// FIXME - find owner of workflow
	// FIXME - get the actual workflow itself and build the request
	// MAYBE: Don't send the workflow within the pubsub, as this requires more data to be sent
	// Check if a worker already exists for company, else run one with:
	// locations, project IDs and subscription names

	// When app is executed:
	// Should update with status execution (somewhere), which will trigger the next node
	// IF action.type == internal, we need the internal watcher to be running and executing
	// This essentially means the WORKER has to be the responsible party for new actions in the INTERNAL landscape
	// Results are ALWAYS posted back to cloud@execution_id?
	if makeNew {
		workflowExecution.Type = "workflow"
		//workflowExecution.Stream = "tmp"
		//workflowExecution.WorkflowQueue = "tmp"
		//workflowExecution.SubscriptionNameNodestream = "testcompany-nodestream"
		workflowExecution.ProjectId = gceProject
		workflowExecution.Locations = []string{"europe-west2"}
		workflowExecution.WorkflowId = workflow.ID
		workflowExecution.StartedAt = int64(time.Now().Unix())
		workflowExecution.CompletedAt = 0
		workflowExecution.Authorization = uuid.NewV4().String()

		// Status for the entire workflow.
		workflowExecution.Status = "EXECUTING"
	}

	if len(workflowExecution.ExecutionSource) == 0 {
		log.Printf("[INFO] No execution source (trigger) specified. Setting to default")
		workflowExecution.ExecutionSource = "default"
	} else {
		log.Printf("[INFO] Execution source is %s for execution ID %s in workflow %s", workflowExecution.ExecutionSource, workflowExecution.ExecutionId, workflowExecution.Workflow.ID)
	}

	workflowExecution.ExecutionVariables = workflow.ExecutionVariables
	// Local authorization for this single workflow used in workers.

	// FIXME: Used for cloud
	//mappedData, err := json.Marshal(workflowExecution)
	//if err != nil {
	//	log.Printf("Failed workflowexecution marshalling: %s", err)
	//	resp.WriteHeader(http.StatusInternalServerError)
	//	resp.Write([]byte(`{"success": false}`))
	//	return
	//}

	//log.Println(string(mappedData))

	if len(workflowExecution.Start) == 0 {
		workflowExecution.Start = workflowExecution.Workflow.Start
	}
	log.Printf("[INFO] New startnode: %s", workflowExecution.Start)

	childNodes := findChildNodes(workflowExecution, workflowExecution.Start)

	topic := "workflows"
	startFound := false
	// FIXME - remove this?
	newActions := []shuffle.Action{}
	defaultResults := []shuffle.ActionResult{}

	allAuths := []shuffle.AppAuthenticationStorage{}
	for _, action := range workflowExecution.Workflow.Actions {
		//action.LargeImage = ""
		if action.ID == workflowExecution.Start {
			startFound = true
		}
		//log.Println(action.Environment)

		if action.Environment == "" {
			return shuffle.WorkflowExecution{}, fmt.Sprintf("Environment is not defined for %s", action.Name), errors.New("Environment not defined!")
		}

		// FIXME: Authentication parameters
		if len(action.AuthenticationId) > 0 {
			if len(allAuths) == 0 {
				allAuths, err = shuffle.GetAllWorkflowAppAuth(ctx, workflow.ExecutingOrg.Id)
				if err != nil {
					log.Printf("Api authentication failed in get all app auth: %s", err)
					return shuffle.WorkflowExecution{}, fmt.Sprintf("Api authentication failed in get all app auth: %s", err), err
				}
			}

			curAuth := shuffle.AppAuthenticationStorage{Id: ""}
			for _, auth := range allAuths {
				if auth.Id == action.AuthenticationId {
					curAuth = auth
					break
				}
			}

			if len(curAuth.Id) == 0 {
				return shuffle.WorkflowExecution{}, fmt.Sprintf("Auth ID %s doesn't exist", action.AuthenticationId), errors.New(fmt.Sprintf("Auth ID %s doesn't exist", action.AuthenticationId))
			}

			// Rebuild params with the right data. This is to prevent issues on the frontend
			newParams := []shuffle.WorkflowAppActionParameter{}
			for _, param := range action.Parameters {

				for _, authparam := range curAuth.Fields {
					if param.Name == authparam.Key {
						param.Value = authparam.Value
						//log.Printf("Name: %s - value: %s", param.Name, param.Value)
						//log.Printf("Name: %s - value: %s\n", param.Name, param.Value)
						break
					}
				}

				newParams = append(newParams, param)
			}

			action.Parameters = newParams
		}

		action.LargeImage = ""
		if len(action.Label) == 0 {
			action.Label = action.ID
		}
		//log.Printf("LABEL: %s", action.Label)
		newActions = append(newActions, action)

		// If the node is NOT found, it's supposed to be set to SKIPPED,
		// as it's not a childnode of the startnode
		// This is a configuration item for the workflow itself.
		if len(workflowExecution.Results) > 0 {
			defaultResults = []shuffle.ActionResult{}
			for _, result := range workflowExecution.Results {
				if result.Status == "WAITING" {
					result.Status = "FINISHED"
					result.Result = "Continuing"
				}

				defaultResults = append(defaultResults, result)
			}
		} else if len(workflowExecution.Results) == 0 && !workflowExecution.Workflow.Configuration.StartFromTop {
			found := false
			for _, nodeId := range childNodes {
				if nodeId == action.ID {
					//log.Printf("Found %s", action.ID)
					found = true
				}
			}

			if !found {
				if action.ID == workflowExecution.Start {
					continue
				}

				//log.Printf("[WARNING] Set %s to SKIPPED as it's NOT a childnode of the startnode.", action.ID)
				curaction := shuffle.Action{
					AppName:    action.AppName,
					AppVersion: action.AppVersion,
					Label:      action.Label,
					Name:       action.Name,
					ID:         action.ID,
				}
				//action
				//curaction.Parameters = []
				defaultResults = append(defaultResults, shuffle.ActionResult{
					Action:        curaction,
					ExecutionId:   workflowExecution.ExecutionId,
					Authorization: workflowExecution.Authorization,
					Result:        "Skipped because it's not under the startnode",
					StartedAt:     0,
					CompletedAt:   0,
					Status:        "SKIPPED",
				})
			}
		}
	}

	for _, trigger := range workflowExecution.Workflow.Triggers {
		//log.Printf("[INFO] ID: %s vs %s", trigger.ID, workflowExecution.Start)
		if trigger.ID == workflowExecution.Start {
			if trigger.AppName == "User Input" {
				startFound = true
				break
			}
		}

		if trigger.AppName == "User Input" || trigger.AppName == "Shuffle Workflow" {
			found := false
			for _, node := range childNodes {
				if node == trigger.ID {
					found = true
					break
				}
			}

			if !found {
				//log.Printf("SHOULD SET TRIGGER %s TO BE SKIPPED", trigger.ID)

				curaction := shuffle.Action{
					AppName:    "shuffle-subflow",
					AppVersion: trigger.AppVersion,
					Label:      trigger.Label,
					Name:       trigger.Name,
					ID:         trigger.ID,
				}

				defaultResults = append(defaultResults, shuffle.ActionResult{
					Action:        curaction,
					ExecutionId:   workflowExecution.ExecutionId,
					Authorization: workflowExecution.Authorization,
					Result:        "Skipped because it's not under the startnode",
					StartedAt:     0,
					CompletedAt:   0,
					Status:        "SKIPPED",
				})
			} else {
				//log.Printf("SHOULD KEEP TRIGGER %s", trigger.ID)
			}
		}
	}
	//childNodes := findChildNodes(workflowExecution, workflowExecution.Start)

	if !startFound {
		log.Printf("[ERROR] Startnode %s doesn't exist!!", workflowExecution.Start)
		return shuffle.WorkflowExecution{}, fmt.Sprintf("Workflow action %s doesn't exist in workflow", workflowExecution.Start), errors.New(fmt.Sprintf(`Workflow start node "%s" doesn't exist. Exiting!`, workflowExecution.Start))
	}

	// Verification for execution environments
	workflowExecution.Results = defaultResults
	workflowExecution.Workflow.Actions = newActions
	onpremExecution := true
	environments := []string{}

	if len(workflowExecution.ExecutionOrg) == 0 && len(workflow.ExecutingOrg.Id) > 0 {
		workflowExecution.ExecutionOrg = workflow.ExecutingOrg.Id
	}

	var allEnvs []shuffle.Environment
	if len(workflowExecution.ExecutionOrg) > 0 {
		//log.Printf("[INFO] Executing ORG: %s", workflowExecution.ExecutionOrg)

		allEnvironments, err := shuffle.GetEnvironments(ctx, workflowExecution.ExecutionOrg)
		if err != nil {
			log.Printf("Failed finding environments: %s", err)
			return shuffle.WorkflowExecution{}, fmt.Sprintf("Workflow environments not found for this org"), errors.New(fmt.Sprintf("Workflow environments not found for this org"))
		}

		for _, curenv := range allEnvironments {
			if curenv.Archived {
				continue
			}

			allEnvs = append(allEnvs, curenv)
		}
	} else {
		log.Printf("[ERROR] No org identified for execution of %s. Returning", workflowExecution.Workflow.ID)
		return shuffle.WorkflowExecution{}, "No org identified for execution", errors.New("No org identified for execution")
	}

	if len(allEnvs) == 0 {
		log.Printf("[ERROR] No active environments found for org: %s", workflowExecution.ExecutionOrg)
		return shuffle.WorkflowExecution{}, "No active environments found", errors.New(fmt.Sprintf("No active env found for org %s", workflowExecution.ExecutionOrg))
	}

	// Check if the actions are children of the startnode?
	imageNames := []string{}
	cloudExec := false
	for _, action := range workflowExecution.Workflow.Actions {
		// Verify if the action environment exists and append
		found := false
		for _, env := range allEnvs {
			if env.Name == action.Environment {
				found = true

				if env.Type == "cloud" {
					cloudExec = true
				} else if env.Type == "onprem" {
					onpremExecution = true
				} else {
					log.Printf("[ERROR] No handler for environment type %s", env.Type)
					return shuffle.WorkflowExecution{}, "No active environments found", errors.New(fmt.Sprintf("No handler for environment type %s", env.Type))
				}
				break
			}
		}

		if !found {
			log.Printf("[ERROR] Couldn't find environment %s. Maybe it's inactive?", action.Environment)
			return shuffle.WorkflowExecution{}, "Couldn't find the environment", errors.New(fmt.Sprintf("Couldn't find env %s in org %s", action.Environment, workflowExecution.ExecutionOrg))
		}

		found = false
		for _, env := range environments {
			if env == action.Environment {

				found = true
				break
			}
		}

		// Check if the app exists?
		newName := action.AppName
		newName = strings.ReplaceAll(newName, " ", "-")
		imageNames = append(imageNames, fmt.Sprintf("%s:%s_%s", baseDockerName, newName, action.AppVersion))

		if !found {
			environments = append(environments, action.Environment)
		}
	}

	err = imageCheckBuilder(imageNames)
	if err != nil {
		log.Printf("[ERROR] Failed building the required images from %#v: %s", imageNames, err)
		return shuffle.WorkflowExecution{}, "Failed building missing Docker images", err
	}

	//b, err := json.Marshal(workflowExecution)
	//if err == nil {
	//	log.Printf("%s", string(b))
	//	log.Printf("LEN: %d", len(string(b)))
	//	//workflowExecution.ExecutionOrg.SyncFeatures = Org{}
	//}

	workflowExecution.Workflow.ExecutingOrg = shuffle.Org{
		Id: workflowExecution.Workflow.ExecutingOrg.Id,
	}
	workflowExecution.Workflow.Org = []shuffle.Org{
		workflowExecution.Workflow.ExecutingOrg,
	}

	//Org               []Org    `json:"org,omitempty" datastore:"org"`
	err = shuffle.SetWorkflowExecution(ctx, workflowExecution, true)
	if err != nil {
		log.Printf("Error saving workflow execution for updates %s: %s", topic, err)
		return shuffle.WorkflowExecution{}, "Failed getting workflowexecution", err
	}

	// Adds queue for onprem execution
	// FIXME - add specifics to executionRequest, e.g. specific environment (can run multi onprem)
	if onpremExecution {
		// FIXME - tmp name based on future companyname-companyId
		// This leads to issues with overlaps. Should set limits and such instead
		for _, environment := range environments {
			log.Printf("[INFO] Execution: %s should execute onprem with execution environment \"%s\". Workflow: %s", workflowExecution.ExecutionId, environment, workflowExecution.Workflow.ID)

			executionRequest := shuffle.ExecutionRequest{
				ExecutionId:   workflowExecution.ExecutionId,
				WorkflowId:    workflowExecution.Workflow.ID,
				Authorization: workflowExecution.Authorization,
				Environments:  environments,
			}

			//executionRequestWrapper, err := getWorkflowQueue(ctx, environment)
			//if err != nil {
			//	executionRequestWrapper = ExecutionRequestWrapper{
			//		Data: []ExecutionRequest{executionRequest},
			//	}
			//} else {
			//	executionRequestWrapper.Data = append(executionRequestWrapper.Data, executionRequest)
			//}

			//log.Printf("Execution request: %#v", executionRequest)
			err = setWorkflowQueue(ctx, executionRequest, environment)
			if err != nil {
				log.Printf("[ERROR] Failed adding execution to db: %s", err)
			}
		}
	}

	// Verifies and runs cloud executions
	if cloudExec {
		featuresList, err := handleVerifyCloudsync(workflowExecution.ExecutionOrg)
		if !featuresList.Workflows.Active || err != nil {
			log.Printf("Error: %s", err)
			log.Printf("[ERROR] Cloud not implemented yet. May need to work on app checking and such")
			return shuffle.WorkflowExecution{}, "Cloud not implemented yet", errors.New("Cloud not implemented yet")
		}

		// What it needs to know:
		// 1. Parameters
		if len(workflowExecution.Workflow.Actions) == 1 {
			log.Printf("Should execute directly with cloud instead of worker because only one action")

			//cloudExecuteAction(workflowExecution.ExecutionId, workflowExecution.Workflow.Actions[0], workflowExecution.ExecutionOrg, workflowExecution.Workflow.ID)
			cloudExecuteAction(workflowExecution)
			return shuffle.WorkflowExecution{}, "Cloud not implemented yet (1)", errors.New("Cloud not implemented yet")
		} else {
			// If it's here, it should be controlled by Worker.
			// If worker, should this backend be a proxy? I think so.
			return shuffle.WorkflowExecution{}, "Cloud not implemented yet (2)", errors.New("Cloud not implemented yet")
		}
	}

	//err = increaseStatisticsField(ctx, "workflow_executions", workflow.ID, 1, workflowExecution.ExecutionOrg)
	//if err != nil {
	//	log.Printf("Failed to increase stats execution stats: %s", err)
	//}

	return workflowExecution, "", nil
}

// This updates stuff locally from remote executions
func cloudExecuteAction(execution shuffle.WorkflowExecution) error {
	ctx := context.Background()
	org, err := shuffle.GetOrg(ctx, execution.ExecutionOrg)
	if err != nil {
		return err
	}

	type ExecutionStruct struct {
		ExecutionId       string                 `json:"execution_id" datastore:"execution_id"`
		Action            shuffle.Action         `json:"action" datastore:"action"`
		Authorization     string                 `json:"authorization" datastore:"authorization"`
		Results           []shuffle.ActionResult `json:"results" datastore:"results,noindex"`
		ExecutionArgument string                 `json:"execution_argument" datastore:"execution_argument,noindex"`
		WorkflowId        string                 `json:"workflow_id" datastore:"workflow_id"`
		ExecutionSource   string                 `json:"execution_source" datastore:"execution_source"`
	}

	data := ExecutionStruct{
		ExecutionId:   execution.ExecutionId,
		WorkflowId:    execution.Workflow.ID,
		Action:        execution.Workflow.Actions[0],
		Authorization: execution.Authorization,
	}
	log.Printf("Executing action: %#v in execution ID %s", data.Action, data.ExecutionId)

	b, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed marshaling api key data: %s", err)
		return err
	}

	syncURL := fmt.Sprintf("%s/api/v1/cloud/sync/execute_node", syncUrl)
	client := &http.Client{}
	req, err := http.NewRequest(
		"POST",
		syncURL,
		bytes.NewBuffer(b),
	)

	req.Header.Add("Authorization", fmt.Sprintf(`Bearer %s`, org.SyncConfig.Apikey))
	newresp, err := client.Do(req)
	if err != nil {
		return err
	}

	respBody, err := ioutil.ReadAll(newresp.Body)
	if err != nil {
		return err
	}

	log.Printf("Finished request. Data: %s", string(respBody))
	log.Printf("Status code: %d", newresp.StatusCode)

	responseData := retStruct{}
	err = json.Unmarshal(respBody, &responseData)
	if err != nil {
		return err
	}

	if newresp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Got status code %d when executing remotely. Expected 200. Contact support.", newresp.StatusCode))
	}

	if !responseData.Success {
		return errors.New(responseData.Reason)
	}

	return nil
}

func executeWorkflow(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("[INFO] Api authentication failed in execute workflow: %s", err)
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

	if len(fileId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID to execute is not valid"}`))
		return
	}

	//memcacheName := fmt.Sprintf("%s_%s", user.Username, fileId)
	ctx := context.Background()
	workflow, err := shuffle.GetWorkflow(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting the workflow locally (execute workflow): %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - have a check for org etc too..
	// FIXME - admin check like this? idk
	if user.Id != workflow.Owner && user.Role != "scheduler" && user.Role != fmt.Sprintf("workflow_%s", fileId) {
		log.Printf("Wrong user (%s) for workflow %s (execute)", user.Username, workflow.ID)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	log.Printf("[INFO] Starting execution of %s!", fileId)

	user.ActiveOrg.Users = []shuffle.User{}
	workflow.ExecutingOrg = user.ActiveOrg
	workflowExecution, executionResp, err := handleExecution(fileId, *workflow, request)

	if err == nil {
		resp.WriteHeader(200)
		resp.Write([]byte(fmt.Sprintf(`{"success": true, "execution_id": "%s", "authorization": "%s"}`, workflowExecution.ExecutionId, workflowExecution.Authorization)))
		return
	}

	resp.WriteHeader(500)
	resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, executionResp)))
}

func stopSchedule(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in schedule workflow: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var fileId string
	var scheduleId string
	if location[1] == "api" {
		if len(location) <= 6 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		fileId = location[4]
		scheduleId = location[6]
	}

	if len(fileId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID to stop schedule is not valid"}`))
		return
	}

	if len(scheduleId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Schedule ID not valid"}`))
		return
	}

	ctx := context.Background()
	workflow, err := shuffle.GetWorkflow(ctx, fileId)
	if err != nil {
		log.Printf("[WARNING] Failed getting the workflow locally (stop schedule): %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - have a check for org etc too..
	// FIXME - admin check like this? idk
	if user.Id != workflow.Owner && user.Role != "scheduler" {
		log.Printf("[WARNING] Wrong user (%s) for workflow %s (stop schedule)", user.Username, workflow.ID)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	schedule, err := shuffle.GetSchedule(ctx, scheduleId)
	if err != nil {
		log.Printf("[WARNING] Failed finding schedule %s", scheduleId)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	//log.Printf("Schedule: %#v", schedule)

	if schedule.Environment == "cloud" {
		log.Printf("[INFO] Should STOP a cloud schedule for workflow %s with schedule ID %s", fileId, scheduleId)
		org, err := shuffle.GetOrg(ctx, user.ActiveOrg.Id)
		if err != nil {
			log.Printf("Failed finding org %s: %s", org.Id, err)
			return
		}

		// 1. Send request to cloud
		// 2. Remove schedule if success
		action := shuffle.CloudSyncJob{
			Type:          "schedule",
			Action:        "stop",
			OrgId:         org.Id,
			PrimaryItemId: scheduleId,
			SecondaryItem: schedule.Frequency,
			ThirdItem:     workflow.ID,
		}

		err = executeCloudAction(action, org.SyncConfig.Apikey)
		if err != nil {
			log.Printf("[WARNING] Failed cloud action STOP schedule: %s", err)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
			return
		} else {
			log.Printf("[INFO] Successfully ran cloud action STOP schedule")
			err = DeleteKey(ctx, "schedules", scheduleId)
			if err != nil {
				log.Printf("[WARNING] Failed deleting cloud schedule onprem..")
				resp.WriteHeader(401)
				resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed deleting cloud schedule"}`)))
				return
			}

			resp.WriteHeader(200)
			resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
			return
		}
	}

	err = deleteSchedule(ctx, scheduleId)
	if err != nil {
		log.Printf("[WARNING] Failed deleting schedule: %s", err)
		if strings.Contains(err.Error(), "Job not found") {
			resp.WriteHeader(200)
			resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
		} else {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed stopping schedule"}`)))
		}
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
	return
}

func stopScheduleGCP(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in schedule workflow: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	location := strings.Split(request.URL.String(), "/")

	var fileId string
	var scheduleId string
	if location[1] == "api" {
		if len(location) <= 6 {
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		fileId = location[4]
		scheduleId = location[6]
	}

	if len(fileId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID to stop schedule is not valid"}`))
		return
	}

	if len(scheduleId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Schedule ID not valid"}`))
		return
	}

	ctx := context.Background()
	workflow, err := shuffle.GetWorkflow(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting the workflow locally (stop schedule GCP): %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - have a check for org etc too..
	// FIXME - admin check like this? idk
	if user.Id != workflow.Owner && user.Role != "scheduler" {
		log.Printf("Wrong user (%s) for workflow %s (stop schedule)", user.Username, workflow.ID)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if len(workflow.Actions) == 0 {
		workflow.Actions = []shuffle.Action{}
	}
	if len(workflow.Branches) == 0 {
		workflow.Branches = []shuffle.Branch{}
	}
	if len(workflow.Triggers) == 0 {
		workflow.Triggers = []shuffle.Trigger{}
	}
	if len(workflow.Errors) == 0 {
		workflow.Errors = []string{}
	}

	err = deleteSchedule(ctx, scheduleId)
	if err != nil {
		if strings.Contains(err.Error(), "Job not found") {
			resp.WriteHeader(200)
			resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
		} else {
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed stopping schedule"}`)))
		}
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
	return
}

func deleteSchedule(ctx context.Context, id string) error {
	log.Printf("Should stop schedule %s!", id)
	err := DeleteKey(ctx, "schedules", id)
	if err != nil {
		log.Printf("Failed to delete schedule: %s", err)
		return err
	} else {
		if value, exists := scheduledJobs[id]; exists {
			log.Printf("STOPPING THIS SCHEDULE: %s", id)
			// Looks like this does the trick? Hurr
			value.Lock()
		} else {
			// FIXME - allow it to kind of stop anyway?
			return errors.New("Can't find the schedule.")
		}
	}

	return nil
}

func deleteScheduleGCP(ctx context.Context, id string) error {
	c, err := scheduler.NewCloudSchedulerClient(ctx)
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	req := &schedulerpb.DeleteJobRequest{
		Name: fmt.Sprintf("projects/%s/locations/europe-west2/jobs/schedule_%s", gceProject, id),
	}

	err = c.DeleteJob(ctx, req)
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	return nil
}

func scheduleWorkflow(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in schedule workflow: %s", err)
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

	if len(fileId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID to start schedule is not valid"}`))
		return
	}

	ctx := context.Background()
	workflow, err := shuffle.GetWorkflow(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting the workflow locally (schedule workflow): %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - have a check for org etc too..
	// FIXME - admin check like this? idk
	if user.Id != workflow.Owner && user.Role != "scheduler" {
		log.Printf("Wrong user (%s) for workflow %s", user.Username, workflow.ID)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if len(workflow.Actions) == 0 {
		workflow.Actions = []shuffle.Action{}
	}
	if len(workflow.Branches) == 0 {
		workflow.Branches = []shuffle.Branch{}
	}
	if len(workflow.Triggers) == 0 {
		workflow.Triggers = []shuffle.Trigger{}
	}
	if len(workflow.Errors) == 0 {
		workflow.Errors = []string{}
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Failed hook unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	var schedule shuffle.Schedule
	err = json.Unmarshal(body, &schedule)
	if err != nil {
		log.Printf("Failed schedule POST unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Finds the startnode for the specific schedule
	startNode := ""
	for _, branch := range workflow.Branches {
		if branch.SourceID == schedule.Id {
			startNode = branch.DestinationID
		}
	}

	if startNode == "" {
		startNode = workflow.Start
	}

	log.Printf("Startnode: %s", startNode)

	if len(schedule.Id) != 36 {
		log.Printf("ID length is not 36 for schedule: %s", err)
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(`{"success": false, "reason": "Invalid data"}`))
		return
	}

	if len(schedule.Name) == 0 {
		log.Printf("Empty name.")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Schedule name can't be empty"}`))
		return
	}

	if len(schedule.Frequency) == 0 {
		log.Printf("Empty frequency.")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Frequency can't be empty"}`))
		return
	}

	scheduleArg, err := json.Marshal(schedule.ExecutionArgument)
	if err != nil {
		log.Printf("Failed scheduleArg marshal: %s", err)
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Clean up garbage. This might be wrong in some very specific use-cases
	parsedBody := string(scheduleArg)
	parsedBody = strings.Replace(parsedBody, "\\\"", "\"", -1)
	if len(parsedBody) > 0 {
		if string(parsedBody[0]) == `"` && string(parsedBody[len(parsedBody)-1]) == "\"" {
			parsedBody = parsedBody[1 : len(parsedBody)-1]
		}
	}

	if schedule.Environment == "cloud" {
		log.Printf("[INFO] Should START a cloud schedule for workflow %s with schedule ID %s", workflow.ID, schedule.Id)
		org, err := shuffle.GetOrg(ctx, user.ActiveOrg.Id)
		if err != nil {
			log.Printf("Failed finding org %s: %s", org.Id, err)
			return
		}

		// 1 = scheduleId
		// 2 = schedule (cron, frequency)
		// 3 = workflowId
		// 4 = execution argument
		action := shuffle.CloudSyncJob{
			Type:          "schedule",
			Action:        "start",
			OrgId:         org.Id,
			PrimaryItemId: schedule.Id,
			SecondaryItem: schedule.Frequency,
			ThirdItem:     workflow.ID,
			FourthItem:    schedule.ExecutionArgument,
			FifthItem:     startNode,
		}

		timeNow := int64(time.Now().Unix())
		newSchedule := ScheduleOld{
			Id:                   schedule.Id,
			WorkflowId:           workflow.ID,
			StartNode:            startNode,
			Argument:             string(schedule.ExecutionArgument),
			WrappedArgument:      parsedBody,
			CreationTime:         timeNow,
			LastModificationtime: timeNow,
			LastRuntime:          timeNow,
			Org:                  org.Id,
			Frequency:            schedule.Frequency,
			Environment:          "cloud",
		}

		err = setSchedule(ctx, newSchedule)
		if err != nil {
			log.Printf("Failed setting cloud schedule: %s", err)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
			return
		}

		log.Printf("Action: %#v", action)
		err = executeCloudAction(action, org.SyncConfig.Apikey)
		if err != nil {
			log.Printf("Failed cloud action START schedule: %s", err)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
			return
		} else {
			log.Printf("Successfully set up cloud action schedule")
			resp.WriteHeader(200)
			resp.Write([]byte(fmt.Sprintf(`{"success": true, "reason": "Done"}`)))
			return
		}
	}

	log.Printf("Schedulearg: %s", parsedBody)

	err = createSchedule(
		ctx,
		schedule.Id,
		workflow.ID,
		schedule.Name,
		startNode,
		schedule.Frequency,
		user.ActiveOrg.Id,
		[]byte(parsedBody),
	)

	// FIXME - real error message lol
	if err != nil {
		log.Printf("Failed creating schedule: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Invalid argument. Try cron */15 * * * *"}`)))
		return
	}

	workflow.Schedules = append(workflow.Schedules, schedule)
	err = shuffle.SetWorkflow(ctx, *workflow, workflow.ID)
	if err != nil {
		log.Printf("Failed setting workflow for schedule: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
	return
}

func setExampleresult(ctx context.Context, result shuffle.AppExecutionExample) error {
	// FIXME: Reintroduce this for stats
	//key := datastore.NameKey("example_result", result.ExampleId, nil)

	//// New struct, to not add body, author etc
	//if _, err := dbclient.Put(ctx, key, &result); err != nil {
	//	log.Printf("Error adding workflow: %s", err)
	//	return err
	//}

	return nil
}

// FIXME: Not suitable for cloud right now :O
func deleteWorkflowApp(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, userErr := shuffle.HandleApiAuthentication(resp, request)
	if userErr != nil {
		log.Printf("Api authentication failed in edit workflow: %s", userErr)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	location := strings.Split(request.URL.String(), "/")
	log.Printf("%#v", location)
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
	log.Printf("ID: %s", fileId)
	app, err := shuffle.GetApp(ctx, fileId, user)
	if err != nil {
		log.Printf("Error getting app (delete) %s: %s", fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - check whether it's in use and maybe restrict again for later?
	// FIXME - actually delete other than private apps too..
	private := false
	if app.Downloaded && user.Role == "admin" {
		log.Printf("[INFO] Deleting downloaded app (authenticated users can do this)")
	} else if user.Id != app.Owner {
		log.Printf("[WARNING] Wrong user (%s) for app %s (delete)", user.Username, app.Name)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	} else {
		log.Printf("[WARNING] App to be deleted is private")
		private = true
	}

	// FIXME: Make workflows track themself INSIDE apps, or with a reference
	q := datastore.NewQuery("workflow").Filter("org_id = ", user.ActiveOrg.Id).Limit(30)
	var workflows []shuffle.Workflow
	_, err = dbclient.GetAll(ctx, q, &workflows)
	if err != nil {
		log.Printf("[WARNING] Failed getting related workflows for the app: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	// Finds workflows using the app to set errors
	// FIXME: this will be WAY too big for cloud :O
	for _, workflow := range workflows {
		found := false

		newActions := []shuffle.Action{}
		for _, action := range workflow.Actions {
			if action.AppName == app.Name && action.AppVersion == app.AppVersion {
				found = true
				action.Errors = append(action.Errors, "App has been deleted")
				action.IsValid = false
			}

			newActions = append(newActions, action)
		}

		if found {
			workflow.IsValid = false
			workflow.Errors = append(workflow.Errors, fmt.Sprintf("App %s_%s has been deleted", app.Name, app.AppVersion))
			workflow.Actions = newActions

			for _, trigger := range workflow.Triggers {
				_ = trigger
				//log.Printf("TRIGGER: %#v", trigger)
				//err = deleteSchedule(ctx, scheduleId)
				//if err != nil {
				//	if strings.Contains(err.Error(), "Job not found") {
				//		resp.WriteHeader(200)
				//		resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
				//	} else {
				//		resp.WriteHeader(401)
				//		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed stopping schedule"}`)))
				//	}
				//	return
				//}
			}

			err = shuffle.SetWorkflow(ctx, workflow, workflow.ID)
			if err != nil {
				log.Printf("Failed setting workflow when deleting app: %s", err)
				continue
			} else {
				log.Printf("Set %s (%s) to have errors", workflow.ID, workflow.Name)
			}

		}

	}

	//resp.WriteHeader(200)
	//resp.Write([]byte(`{"success": true}`))
	//return

	// Not really deleting it, just removing from user cache
	if private {
		log.Printf("[INFO] Deleting private app")
		var privateApps []shuffle.WorkflowApp
		for _, item := range user.PrivateApps {
			if item.ID == fileId {
				continue
			}

			privateApps = append(privateApps, item)
		}

		user.PrivateApps = privateApps
		err = shuffle.SetUser(ctx, &user)
		if err != nil {
			log.Printf("[ERROR] Failed removing %s app for user %s: %s", app.Name, user.Username, err)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": true"}`)))
			return
		}
	}

	log.Printf("[INFO] Deleting public app")
	err = DeleteKey(ctx, "workflowapp", fileId)
	if err != nil {
		log.Printf("Failed deleting workflowapp")
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed deleting workflow app"}`)))
		return
	}

	err = increaseStatisticsField(ctx, "total_apps_deleted", fileId, 1, user.ActiveOrg.Id)
	if err != nil {
		log.Printf("Failed to increase total apps loaded stats: %s", err)
	}
	cacheKey := fmt.Sprintf("workflowapps-sorted-100")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-500")
	shuffle.DeleteCache(ctx, cacheKey)

	//err = memcache.Delete(request.Context(), sessionToken)
	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true}`))
}

func getWorkflowAppConfig(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	ctx := context.Background()

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

	app, err := shuffle.GetApp(ctx, fileId, shuffle.User{})
	if err != nil {
		log.Printf("[WARNING] Error getting app %s (app config): %s", fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "App doesn't exist"}`))
		return
	}

	//if IsValid       bool   `json:"is_valid" yaml:"is_valid" required:true datastore:"is_valid"`
	// Sharing       bool   `json:"sharing" yaml:"sharing" required:false datastore:"sharing"`
	//log.Printf("Sharing: %s", app.Sharing)
	//log.Printf("Generated: %s", app.Generated)
	//log.Printf("Downloaded: %s", app.Downloaded)

	// FIXME - Handle sharing and such PROPERLY
	if app.Sharing && app.Generated {
		log.Printf("CAN SHARE APP!")
		parsedApi, err := getOpenApiDatastore(ctx, fileId)
		if err != nil {
			log.Printf("[WARNING] OpenApi doesn't exist for: %s - err: %s", fileId, err)
			resp.WriteHeader(401)
			resp.Write([]byte(`{"success": false}`))
			return
		}

		if len(parsedApi.ID) > 0 {
			parsedApi.Success = true
		} else {
			parsedApi.Success = false
		}

		//log.Printf("PARSEDAPI: %#v", parsedApi)
		data, err := json.Marshal(parsedApi)
		if err != nil {
			log.Printf("[WARNING] Error parsing api json: %s", err)
			resp.WriteHeader(422)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed marshalling new parsed swagger: %s"}`, err)))
			return
		}

		resp.WriteHeader(200)
		resp.Write(data)
		return
	}

	user, userErr := shuffle.HandleApiAuthentication(resp, request)
	if userErr != nil {
		log.Printf("[WARNING] Api authentication failed in get app: %s", userErr)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Id != app.Owner {
		log.Printf("[WARNING] Wrong user (%s) for app %s", user.Username, app.Name)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	log.Printf("[INFO] Getting app %s (OpenAPI)", fileId)
	parsedApi, err := getOpenApiDatastore(ctx, fileId)
	if err != nil {
		log.Printf("OpenApi doesn't exist for: %s - err: %s", fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	//log.Printf("Parsed API: %#v", parsedApi)
	if len(parsedApi.ID) > 0 {
		parsedApi.Success = true
	} else {
		parsedApi.Success = false
	}

	data, err := json.Marshal(parsedApi)
	if err != nil {
		resp.WriteHeader(422)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed marshalling new parsed swagger: %s"}`, err)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(data)
}

func getWorkflowApps(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// FIXME - set this to be per user IF logged in,
	// as there might exist private and public
	//memcacheName := "all_apps"

	ctx := context.Background()
	// Just need to be logged in
	// FIXME - need to be logged in?
	user, userErr := shuffle.HandleApiAuthentication(resp, request)
	if userErr != nil {
		log.Printf("Continuing with apps even without auth")
		//log.Printf("Api authentication failed in get all apps: %s", userErr)
		//resp.WriteHeader(401)
		//resp.Write([]byte(`{"success": false}`))
		//return
	}

	//if item, err := memcache.Get(ctx, memcacheName); err == memcache.ErrCacheMiss {
	//	// Not in cache
	//	log.Printf("Apps not in cache.")
	//} else if err != nil {
	//	log.Printf("Error getting item: %v", err)
	//} else {
	//	// FIXME - verify if value is ok? Can unmarshal etc.
	//	allApps := item.Value

	//	if userErr == nil && len(user.PrivateApps) > 0 {
	//		var parsedApps []WorkflowApp
	//		err = json.Unmarshal(allApps, &parsedApps)
	//		if err == nil {
	//			log.Printf("Shouldve added %d apps", len(user.PrivateApps))
	//			user.PrivateApps = append(user.PrivateApps, parsedApps...)

	//			tmpApps, err := json.Marshal(user.PrivateApps)
	//			if err == nil {
	//				allApps = tmpApps
	//			}
	//		}
	//	}

	//	resp.WriteHeader(200)
	//	resp.Write(allApps)
	//	return
	//}

	workflowapps, err := shuffle.GetAllWorkflowApps(ctx, 500)
	if err != nil {
		log.Printf("Failed getting apps (getworkflowapps): %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}
	//log.Printf("Length: %d", len(workflowapps))

	// FIXME - this is really garbage, but is here to protect again null values etc.

	newapps := workflowapps
	/*
		skipApps := []string{"Shuffle Subflow"}
		newapps := []WorkflowApp{}
		baseApps := []WorkflowApp{}
			for _, workflowapp := range workflowapps {
				//if !workflowapp.Activated && workflowapp.Generated {
				//	continue
				//}

				if workflowapp.Owner != user.Id && user.Role != "admin" && !workflowapp.Sharing {
					continue
				}

				continueOuter := false
				for _, skip := range skipApps {
					if workflowapp.Name == skip {
						continueOuter = true
						break
					}
				}

				if continueOuter {
					continue
				}

				//workflowapp.Environment = "cloud"
				newactions := []WorkflowAppAction{}
				for _, action := range workflowapp.Actions {
					//action.Environment = workflowapp.Environment
					if len(action.Parameters) == 0 {
						action.Parameters = []WorkflowAppActionParameter{}
					}

					newactions = append(newactions, action)
				}

				workflowapp.Actions = newactions
				newapps = append(newapps, workflowapp)
				baseApps = append(baseApps, workflowapp)
			}
	*/

	if len(user.PrivateApps) > 0 {
		found := false
		for _, item := range user.PrivateApps {
			for _, app := range newapps {
				if item.ID == app.ID {
					found = true
					break
				}
			}

			if !found {
				newapps = append(newapps, item)
			}
		}
	}

	// Double unmarshal because of user apps
	newbody, err := json.Marshal(newapps)
	//newbody, err := json.Marshal(workflowapps)
	if err != nil {
		log.Printf("Failed unmarshalling all newapps: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking workflow apps"}`)))
		return
	}

	//basebody, err := json.Marshal(baseApps)
	////newbody, err := json.Marshal(workflowapps)
	//if err != nil {
	//	log.Printf("Failed unmarshalling all baseapps: %s", err)
	//	resp.WriteHeader(401)
	//	resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking workflow apps"}`)))
	//	return
	//}

	// Refreshed every hour
	//item := &memcache.Item{
	//	Key:        memcacheName,
	//	Value:      basebody,
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

	//log.Println(string(body))
	//log.Println(string(newbody))
	resp.WriteHeader(200)
	resp.Write(newbody)
}

// Bad check for workflowapps :)
// FIXME - use tags and struct reflection
func checkWorkflowApp(workflowApp shuffle.WorkflowApp) error {
	// Validate fields
	if workflowApp.Name == "" {
		return errors.New("App field name doesn't exist")
	}

	if workflowApp.Description == "" {
		return errors.New("App field description doesn't exist")
	}

	if workflowApp.AppVersion == "" {
		return errors.New("App field app_version doesn't exist")
	}

	if workflowApp.ContactInfo.Name == "" {
		return errors.New("App field contact_info.name doesn't exist")
	}

	return nil
}

func handleGetfile(resp http.ResponseWriter, request *http.Request) ([]byte, error) {
	// Upload file here first
	request.ParseMultipartForm(32 << 20)
	file, _, err := request.FormFile("file")
	if err != nil {
		log.Printf("Error parsing: %s", err)
		return []byte{}, err
	}
	defer file.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		return []byte{}, err
	}

	return buf.Bytes(), nil
}

// Basically a search for apps that aren't activated yet
func getSpecificApps(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// Just need to be logged in
	// FIXME - should have some permissions?
	_, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in set new app: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Error with body read: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	type tmpStruct struct {
		Search string `json:"search"`
	}

	var tmpBody tmpStruct
	err = json.Unmarshal(body, &tmpBody)
	if err != nil {
		log.Printf("Error with unmarshal tmpBody: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - continue the search here with github repos etc.
	// Caching might be smart :D
	ctx := context.Background()
	workflowapps, err := shuffle.GetAllWorkflowApps(ctx, 500)
	if err != nil {
		log.Printf("Error: Failed getting workflowapps: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	returnValues := []shuffle.WorkflowApp{}
	search := strings.ToLower(tmpBody.Search)
	for _, app := range workflowapps {
		if !app.Activated && app.Generated {
			// This might be heavy with A LOT
			// Not too worried with todays tech tbh..
			appName := strings.ToLower(app.Name)
			appDesc := strings.ToLower(app.Description)
			if strings.Contains(appName, search) || strings.Contains(appDesc, search) {
				//log.Printf("Name: %s, Generated: %s, Activated: %s", app.Name, strconv.FormatBool(app.Generated), strconv.FormatBool(app.Activated))
				returnValues = append(returnValues, app)
			}
		}
	}

	newbody, err := json.Marshal(returnValues)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking workflow executions"}`)))
		return
	}

	returnData := fmt.Sprintf(`{"success": true, "reason": %s}`, string(newbody))
	resp.WriteHeader(200)
	resp.Write([]byte(returnData))
}

func validateAppInput(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// Just need to be logged in
	// FIXME - should have some permissions?
	_, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in set new app: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	filebytes, err := handleGetfile(resp, request)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	kind, err := filetype.Match(filebytes)
	if err != nil {
		log.Printf("Failed parsing filetype")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	//fmt.Printf("File type: %s. MIME: %s\n", kind.Extension, kind.MIME.Value)
	if kind == filetype.Unknown {
		fmt.Println("Unknown file type")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if kind.MIME.Value != "application/zip" {
		fmt.Println("Not zip, can't unzip")
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - validate folderstructure, Dockerfile, python scripts, api.yaml, requirements.txt, src/

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

// Deploy to google cloud function :)
func deployCloudFunctionPython(ctx context.Context, name, localization, applocation string, environmentVariables map[string]string) error {
	service, err := cloudfunctions.NewService(ctx)
	if err != nil {
		return err
	}

	// ProjectsLocationsListCall
	projectsLocationsFunctionsService := cloudfunctions.NewProjectsLocationsFunctionsService(service)
	location := fmt.Sprintf("projects/%s/locations/%s", gceProject, localization)
	functionName := fmt.Sprintf("%s/functions/%s", location, name)

	cloudFunction := &cloudfunctions.CloudFunction{
		AvailableMemoryMb:    128,
		EntryPoint:           "authorization",
		EnvironmentVariables: environmentVariables,
		HttpsTrigger:         &cloudfunctions.HttpsTrigger{},
		MaxInstances:         0,
		Name:                 functionName,
		Runtime:              "python37",
		SourceArchiveUrl:     applocation,
	}

	//getCall := projectsLocationsFunctionsService.Get(fmt.Sprintf("%s/functions/function-5", location))
	//resp, err := getCall.Do()

	createCall := projectsLocationsFunctionsService.Create(location, cloudFunction)
	_, err = createCall.Do()
	if err != nil {
		log.Printf("Failed creating new function. SKIPPING patch, as it probably already exists: %s", err)

		// FIXME - have patching code or nah?
		createCall := projectsLocationsFunctionsService.Patch(fmt.Sprintf("%s/functions/%s", location, name), cloudFunction)
		_, err = createCall.Do()
		if err != nil {
			log.Println("Failed patching function")
			return err
		}

		log.Printf("Successfully patched %s to %s", name, localization)
	} else {
		log.Printf("Successfully deployed %s to %s", name, localization)
	}

	// FIXME - use response to define the HTTPS entrypoint. It's default to an easy one tho

	return nil
}

// Deploy to google cloud function :)
func deployCloudFunctionGo(ctx context.Context, name, localization, applocation string, environmentVariables map[string]string) error {
	service, err := cloudfunctions.NewService(ctx)
	if err != nil {
		return err
	}

	// ProjectsLocationsListCall
	projectsLocationsFunctionsService := cloudfunctions.NewProjectsLocationsFunctionsService(service)
	location := fmt.Sprintf("projects/%s/locations/%s", gceProject, localization)
	functionName := fmt.Sprintf("%s/functions/%s", location, name)

	cloudFunction := &cloudfunctions.CloudFunction{
		AvailableMemoryMb:    128,
		EntryPoint:           "Authorization",
		EnvironmentVariables: environmentVariables,
		HttpsTrigger:         &cloudfunctions.HttpsTrigger{},
		MaxInstances:         1,
		Name:                 functionName,
		Runtime:              "go111",
		SourceArchiveUrl:     applocation,
	}

	//getCall := projectsLocationsFunctionsService.Get(fmt.Sprintf("%s/functions/function-5", location))
	//resp, err := getCall.Do()

	createCall := projectsLocationsFunctionsService.Create(location, cloudFunction)
	_, err = createCall.Do()
	if err != nil {
		log.Println("Failed creating new function. Attempting patch, as it might exist already")

		createCall := projectsLocationsFunctionsService.Patch(fmt.Sprintf("%s/functions/%s", location, name), cloudFunction)
		_, err = createCall.Do()
		if err != nil {
			log.Println("Failed patching function")
			return err
		}

		log.Printf("Successfully patched %s to %s", name, localization)
	} else {
		log.Printf("Successfully deployed %s to %s", name, localization)
	}

	// FIXME - use response to define the HTTPS entrypoint. It's default to an easy one tho

	return nil
}

// Deploy to google cloud function :)
func deployWebhookFunction(ctx context.Context, name, localization, applocation string, environmentVariables map[string]string) error {
	service, err := cloudfunctions.NewService(ctx)
	if err != nil {
		return err
	}

	// ProjectsLocationsListCall
	projectsLocationsFunctionsService := cloudfunctions.NewProjectsLocationsFunctionsService(service)
	location := fmt.Sprintf("projects/%s/locations/%s", gceProject, localization)
	functionName := fmt.Sprintf("%s/functions/%s", location, name)

	cloudFunction := &cloudfunctions.CloudFunction{
		AvailableMemoryMb:    128,
		EntryPoint:           "Authorization",
		EnvironmentVariables: environmentVariables,
		HttpsTrigger:         &cloudfunctions.HttpsTrigger{},
		MaxInstances:         1,
		Name:                 functionName,
		Runtime:              "go111",
		SourceArchiveUrl:     applocation,
	}

	//getCall := projectsLocationsFunctionsService.Get(fmt.Sprintf("%s/functions/function-5", location))
	//resp, err := getCall.Do()

	createCall := projectsLocationsFunctionsService.Create(location, cloudFunction)
	_, err = createCall.Do()
	if err != nil {
		log.Println("Failed creating new function. Attempting patch, as it might exist already")

		createCall := projectsLocationsFunctionsService.Patch(fmt.Sprintf("%s/functions/%s", location, name), cloudFunction)
		_, err = createCall.Do()
		if err != nil {
			log.Println("Failed patching function")
			return err
		}

		log.Printf("Successfully patched %s to %s", name, localization)
	} else {
		log.Printf("Successfully deployed %s to %s", name, localization)
	}

	// FIXME - use response to define the HTTPS entrypoint. It's default to an easy one tho

	return nil
}

func loadGithubWorkflows(url, username, password, userId, branch, orgId string) error {
	fs := memfs.New()

	log.Printf("Starting load of %s with branch %s", url, branch)
	if strings.Contains(url, "github") || strings.Contains(url, "gitlab") || strings.Contains(url, "bitbucket") {
		cloneOptions := &git.CloneOptions{
			URL: url,
		}

		// FIXME: Better auth.
		if len(username) > 0 && len(password) > 0 {
			cloneOptions.Auth = &http2.BasicAuth{

				Username: username,
				Password: password,
			}
		}

		// main is the new master
		if len(branch) > 0 && branch != "main" && branch != "master" {
			cloneOptions.ReferenceName = plumbing.ReferenceName(branch)
		}

		storer := memory.NewStorage()
		r, err := git.Clone(storer, fs, cloneOptions)
		if err != nil {
			log.Printf("Failed loading repo %s into memory (github workflows): %s", url, err)
			return err
		}

		dir, err := fs.ReadDir("/")
		if err != nil {
			log.Printf("FAiled reading folder: %s", err)
		}
		_ = r

		log.Printf("Starting workflow folder iteration")
		iterateWorkflowGithubFolders(fs, dir, "", "", userId, orgId)

	} else if strings.Contains(url, "s3") {
		//https://docs.aws.amazon.com/sdk-for-go/api/service/s3/

		//sess := session.Must(session.NewSession())
		//downloader := s3manager.NewDownloader(sess)

		//// Write the contents of S3 Object to the file
		//storer := memory.NewStorage()
		//n, err := downloader.Download(storer, &s3.GetObjectInput{
		//	Bucket: aws.String(myBucket),
		//	Key:    aws.String(myString),
		//})
		//if err != nil {
		//	return fmt.Errorf("failed to download file, %v", err)
		//}
		//fmt.Printf("file downloaded, %d bytes\n", n)
	} else {
		return errors.New(fmt.Sprintf("URL %s is unsupported when downloading workflows", url))
	}

	return nil
}

func loadSpecificWorkflows(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// Just need to be logged in
	// FIXME - should have some permissions?
	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in load apps: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Role != "admin" {
		log.Printf("Wrong user (%s) when downloading from github", user.Username)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Downloading remotely requires admin"}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Error with body read: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Field1 & 2 can be a lot of things..
	type tmpStruct struct {
		URL    string `json:"url"`
		Field1 string `json:"field_1"`
		Field2 string `json:"field_2"`
		Field3 string `json:"field_3"`
	}
	//log.Printf("Body: %s", string(body))

	var tmpBody tmpStruct
	err = json.Unmarshal(body, &tmpBody)
	if err != nil {
		log.Printf("Error with unmarshal tmpBody: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Field3 = branch
	err = loadGithubWorkflows(tmpBody.URL, tmpBody.Field1, tmpBody.Field2, user.Id, tmpBody.Field3, user.ActiveOrg.Id)
	if err != nil {
		log.Printf("Failed to update workflows: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

func handleAppHotloadRequest(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	ctx := context.Background()
	cacheKey := fmt.Sprintf("workflowapps-sorted-100")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-500")
	shuffle.DeleteCache(ctx, cacheKey)

	// Just need to be logged in
	// FIXME - should have some permissions?
	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in app hotload: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Role != "admin" {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Must be admin to hotload apps"}`))
		return
	}

	location := os.Getenv("SHUFFLE_APP_HOTLOAD_FOLDER")
	if len(location) == 0 {
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "SHUFFLE_APP_HOTLOAD_FOLDER not specified in .env"}`)))
		return
	}

	log.Printf("[INFO] Starting hotloading from %s", location)
	err = handleAppHotload(ctx, location, true)
	if err != nil {
		log.Printf("[WARNING] Failed app hotload: %s", err)
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	cacheKey = fmt.Sprintf("workflowapps-sorted-100")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-500")
	shuffle.DeleteCache(ctx, cacheKey)

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

func loadSpecificApps(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// Just need to be logged in
	// FIXME - should have some permissions?
	_, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in load specific apps: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Error with body read: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Field1 & 2 can be a lot of things.
	// Field1 = Username
	// Field2 = Password
	type tmpStruct struct {
		URL         string `json:"url"`
		Branch      string `json:"branch"`
		Field1      string `json:"field_1"`
		Field2      string `json:"field_2"`
		ForceUpdate bool   `json:"force_update"`
	}
	//log.Printf("Body: %s", string(body))

	var tmpBody tmpStruct
	err = json.Unmarshal(body, &tmpBody)
	if err != nil {
		log.Printf("Error with unmarshal tmpBody: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	fs := memfs.New()

	if strings.Contains(tmpBody.URL, "github") || strings.Contains(tmpBody.URL, "gitlab") || strings.Contains(tmpBody.URL, "bitbucket") {
		cloneOptions := &git.CloneOptions{
			URL: tmpBody.URL,
		}

		if len(tmpBody.Branch) > 0 && tmpBody.Branch != "master" && tmpBody.Branch != "main" {
			cloneOptions.ReferenceName = plumbing.ReferenceName(tmpBody.Branch)
		}

		// FIXME: Better auth.
		if len(tmpBody.Field1) > 0 && len(tmpBody.Field2) > 0 {
			cloneOptions.Auth = &http2.BasicAuth{
				Username: tmpBody.Field1,
				Password: tmpBody.Field2,
			}
		}

		storer := memory.NewStorage()
		r, err := git.Clone(storer, fs, cloneOptions)
		if err != nil {
			log.Printf("Failed loading repo %s into memory (github workflows 2): %s", tmpBody.URL, err)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
			return
		}

		dir, err := fs.ReadDir("/")
		if err != nil {
			log.Printf("FAiled reading folder: %s", err)
		}
		_ = r

		if tmpBody.ForceUpdate {
			log.Printf("Running with force update!")
		} else {
			log.Printf("Updating apps with updates")
		}

		if tmpBody.ForceUpdate {
			ctx := context.Background()
			dockercli, err := client.NewEnvClient()
			if err == nil {
				_, err := dockercli.ImagePull(ctx, "frikky/shuffle:app_sdk", types.ImagePullOptions{})
				if err != nil {
					log.Printf("[WARNING] Failed to download apps with the new App SDK: %s", err)
				}
			} else {
				log.Printf("[WARNING] Failed to download apps with the new App SDK because of docker cli: %s", err)
			}
		}

		iterateAppGithubFolders(fs, dir, "", "", tmpBody.ForceUpdate)

	} else if strings.Contains(tmpBody.URL, "s3") {
		//https://docs.aws.amazon.com/sdk-for-go/api/service/s3/

		//sess := session.Must(session.NewSession())
		//downloader := s3manager.NewDownloader(sess)

		//// Write the contents of S3 Object to the file
		//storer := memory.NewStorage()
		//n, err := downloader.Download(storer, &s3.GetObjectInput{
		//	Bucket: aws.String(myBucket),
		//	Key:    aws.String(myString),
		//})
		//if err != nil {
		//	return fmt.Errorf("failed to download file, %v", err)
		//}
		//fmt.Printf("file downloaded, %d bytes\n", n)
	} else {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s is unsupported"}`, tmpBody.URL)))
		return
	}

	ctx := context.Background()
	cacheKey := fmt.Sprintf("workflowapps-sorted-100")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-500")
	shuffle.DeleteCache(ctx, cacheKey)

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

func iterateOpenApiGithub(fs billy.Filesystem, dir []os.FileInfo, extra string, onlyname string) error {

	ctx := context.Background()
	workflowapps, err := shuffle.GetAllWorkflowApps(ctx, 500)
	appCounter := 0
	if err != nil {
		log.Printf("Failed to get existing generated apps")
	}
	for _, file := range dir {
		if len(onlyname) > 0 && file.Name() != onlyname {
			continue
		}

		// Folder?
		switch mode := file.Mode(); {
		case mode.IsDir():
			tmpExtra := fmt.Sprintf("%s%s/", extra, file.Name())
			//log.Printf("TMPEXTRA: %s", tmpExtra)
			dir, err := fs.ReadDir(tmpExtra)
			if err != nil {
				log.Printf("Failed reading dir in openapi: %s", err)
				continue
			}

			// Go routine? Hmm, this can be super quick I guess
			err = iterateOpenApiGithub(fs, dir, tmpExtra, "")
			if err != nil {
				log.Printf("Failed recursion in openapi: %s", err)
				continue
				//break
			}
		case mode.IsRegular():
			// Check the file
			filename := file.Name()
			filteredNames := []string{"FUNDING.yml"}
			if strings.Contains(filename, "yaml") || strings.Contains(filename, "yml") {

				contOuter := false
				for _, name := range filteredNames {
					if filename == name {
						contOuter = true
						break
					}
				}

				if contOuter {
					//log.Printf("Skipping %s", filename)
					continue
				}

				//log.Printf("File: %s", filename)
				//log.Printf("Found file: %s", filename)
				//log.Printf("OpenAPI app: %s", filename)
				tmpExtra := fmt.Sprintf("%s%s/", extra, file.Name())

				fileReader, err := fs.Open(tmpExtra)
				if err != nil {
					continue
				}

				readFile, err := ioutil.ReadAll(fileReader)
				if err != nil {
					log.Printf("Filereader error yaml for %s: %s", filename, err)
					continue
				}

				// 1. This parses OpenAPI v2 to v3 etc, for use.
				parsedOpenApi, err := handleSwaggerValidation(readFile)
				if err != nil {
					log.Printf("Validation error for %s: %s", filename, err)
					continue
				}

				// 2. With parsedOpenApi.ID:
				//http://localhost:3000/apps/new?id=06b1376f77b0563a3b1747a3a1253e88

				// 3. Load this as a "standby" app
				// FIXME: This should be a function ROFL
				//log.Printf("%s", string(readFile))
				swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData([]byte(parsedOpenApi.Body))
				if err != nil {
					log.Printf("Swagger validation error in loop (%s): %s", filename, err)
					continue
				}

				if strings.Contains(swagger.Info.Title, " ") {
					strings.Replace(swagger.Info.Title, " ", "", -1)
				}

				//log.Printf("Should generate yaml")
				swagger, api, _, err := shuffle.GenerateYaml(swagger, parsedOpenApi.ID)
				if err != nil {
					log.Printf("Failed building and generating yaml in loop (%s): %s", filename, err)
					continue
				}

				// FIXME: Configure user?
				api.Owner = ""
				api.ID = parsedOpenApi.ID
				api.IsValid = true
				api.Generated = true
				api.Activated = false

				found := false
				for _, app := range workflowapps {
					if app.ID == api.ID {
						found = true
						break
					} else if app.Name == api.Name && app.AppVersion == api.AppVersion {
						found = true
						break
					}
				}

				if !found {
					err = shuffle.SetWorkflowAppDatastore(ctx, api, api.ID)
					if err != nil {
						log.Printf("Failed setting workflowapp in loop: %s", err)
						continue
					} else {
						appCounter += 1
						log.Printf("Added %s:%s to the database from OpenAPI repo", api.Name, api.AppVersion)

						// Set OpenAPI datastore
						err = setOpenApiDatastore(ctx, parsedOpenApi.ID, parsedOpenApi)
						if err != nil {
							log.Printf("Failed uploading openapi to datastore in loop: %s", err)
							continue
						}

						cacheKey := fmt.Sprintf("workflowapps-sorted-100")
						shuffle.DeleteCache(ctx, cacheKey)
						cacheKey = fmt.Sprintf("workflowapps-sorted-500")
						shuffle.DeleteCache(ctx, cacheKey)
					}
				} else {
					//log.Printf("Skipped upload of %s (%s)", api.Name, api.ID)
				}

				//return nil
			}
		}
	}

	if appCounter > 0 {
		log.Printf("Preloaded %d OpenApi apps in folder %s!", appCounter, extra)
	}

	return nil
}

// Onlyname is used to
func iterateWorkflowGithubFolders(fs billy.Filesystem, dir []os.FileInfo, extra string, onlyname, userId, orgId string) error {
	var err error
	secondsOffset := 0

	// sort file names
	filenames := []string{}
	for _, file := range dir {
		filename := file.Name()
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)

	// iterate through sorted filenames
	for _, filename := range filenames {
		secondsOffset -= 10
		if len(onlyname) > 0 && filename != onlyname {
			continue
		}

		file, err := fs.Stat(filename)
		if err != nil {
			continue
		}

		// Folder?
		switch mode := file.Mode(); {
		case mode.IsDir():
			tmpExtra := fmt.Sprintf("%s%s/", extra, file.Name())
			dir, err := fs.ReadDir(tmpExtra)
			if err != nil {
				log.Printf("Failed to read dir: %s", err)
				continue
			}

			// Go routine? Hmm, this can be super quick I guess
			err = iterateWorkflowGithubFolders(fs, dir, tmpExtra, "", userId, orgId)
			if err != nil {
				continue
			}
		case mode.IsRegular():
			// Check the file
			if strings.HasSuffix(filename, ".json") {
				path := fmt.Sprintf("%s%s", extra, file.Name())
				fileReader, err := fs.Open(path)
				if err != nil {
					log.Printf("Error reading file: %s", err)
					continue
				}

				readFile, err := ioutil.ReadAll(fileReader)
				if err != nil {
					log.Printf("Error reading file: %s", err)
					continue
				}

				var workflow shuffle.Workflow
				err = json.Unmarshal(readFile, &workflow)
				if err != nil {
					continue
				}

				// rewrite owner to user who imports now
				if userId != "" {
					workflow.Owner = userId
				}

				workflow.ID = uuid.NewV4().String()
				workflow.OrgId = orgId
				workflow.ExecutingOrg = shuffle.Org{
					Id: orgId,
				}

				workflow.Org = append(workflow.Org, shuffle.Org{
					Id: orgId,
				})
				workflow.IsValid = false
				workflow.Errors = []string{"Imported, not locally saved. Save before using."}

				/*
					// Find existing similar ones
					q = datastore.NewQuery("workflow").Filter("org_id =", user.ActiveOrg.Id).Filter("name", workflow.name)
					var workflows []Workflow
					_, err = dbclient.GetAll(ctx, q, &workflows)
					if err == nil {
						log.Printf("Failed getting workflows for user %s: %s", user.Username, err)
						if len(workflows) == 0 {
							resp.WriteHeader(200)
							resp.Write([]byte("[]"))
							return
						}
					}
				*/

				log.Printf("Import workflow from file: %s", filename)
				ctx := context.Background()
				err = shuffle.SetWorkflow(ctx, workflow, workflow.ID, secondsOffset)
				if err != nil {
					log.Printf("Failed setting (download) workflow: %s", err)
					continue
				}

				log.Printf("Uploaded workflow %s for user %s and org %s!", filename, userId, orgId)
			}
		}
	}

	return err
}

type buildLaterStruct struct {
	Tags  []string
	Extra string
	Id    string
}

// Onlyname is used to
func iterateAppGithubFolders(fs billy.Filesystem, dir []os.FileInfo, extra string, onlyname string, forceUpdate bool) ([]buildLaterStruct, []buildLaterStruct, error) {
	var err error

	allapps := []shuffle.WorkflowApp{}

	// These are slow apps to build with some funky mechanisms
	reservedNames := []string{
		"OWA",
		"NLP",
		"YARA",
	}

	// It's here to prevent getting them in every iteration
	buildLaterFirst := []buildLaterStruct{}
	buildLaterList := []buildLaterStruct{}
	ctx := context.Background()
	for _, file := range dir {
		if len(onlyname) > 0 && file.Name() != onlyname {
			continue
		}

		// Folder?
		switch mode := file.Mode(); {
		case mode.IsDir():
			tmpExtra := fmt.Sprintf("%s%s/", extra, file.Name())
			dir, err := fs.ReadDir(tmpExtra)
			if err != nil {
				log.Printf("Failed to read dir: %s", err)
				continue
			}

			// Go routine? Hmm, this can be super quick I guess
			buildFirst, buildLast, err := iterateAppGithubFolders(fs, dir, tmpExtra, "", forceUpdate)

			for _, item := range buildFirst {
				buildLaterFirst = append(buildLaterFirst, item)
			}

			for _, item := range buildLast {
				buildLaterList = append(buildLaterList, item)
			}

			if err != nil {
				log.Printf("[WARNING] Error reading folder: %s", err)
				//buildFirst, buildLast, err := iterateAppGithubFolders(fs, dir, tmpExtra, "", forceUpdate)

				if !forceUpdate {
					return buildLaterFirst, buildLaterList, err
				}
			}

		case mode.IsRegular():
			// Check the file
			filename := file.Name()
			if filename == "Dockerfile" {
				// Set up to make md5 and check if the app is new (api.yaml+src/app.py+Dockerfile)
				// Check if Dockerfile, app.py or api.yaml has changed. Hash?
				//log.Printf("Handle Dockerfile in location %s", extra)
				// Try api.yaml and api.yml
				fullPath := fmt.Sprintf("%s%s", extra, "api.yaml")
				fileReader, err := fs.Open(fullPath)
				if err != nil {
					fullPath = fmt.Sprintf("%s%s", extra, "api.yml")
					fileReader, err = fs.Open(fullPath)
					if err != nil {
						log.Printf("Failed finding api.yaml/yml: %s", err)
						continue
					}
				}

				//log.Printf("HANDLING DOCKER FILEREADER - SEARCH&REPLACE?")

				appfileData, err := ioutil.ReadAll(fileReader)
				if err != nil {
					log.Printf("Failed reading %s: %s", fullPath, err)
					continue
				}

				if len(appfileData) == 0 {
					log.Printf("Failed reading %s - length is 0.", fullPath)
					continue
				}

				// func md5sum(data []byte) string {
				// Make hash
				appPython := fmt.Sprintf("%s/src/app.py", extra)
				appPythonReader, err := fs.Open(appPython)
				if err != nil {
					log.Printf("Failed to read %s", appPython)
					continue
				}

				appPythonData, err := ioutil.ReadAll(appPythonReader)
				if err != nil {
					log.Printf("Failed reading %s: %s", appPython, err)
					continue
				}

				dockerFp := fmt.Sprintf("%s/Dockerfile", extra)
				dockerfile, err := fs.Open(dockerFp)
				if err != nil {
					log.Printf("Failed to read %s", appPython)
					continue
				}

				dockerfileData, err := ioutil.ReadAll(dockerfile)
				if err != nil {
					log.Printf("Failed to read dockerfile")
					continue
				}

				combined := []byte{}
				combined = append(combined, appfileData...)
				combined = append(combined, appPythonData...)
				combined = append(combined, dockerfileData...)
				md5 := md5sum(combined)

				var workflowapp shuffle.WorkflowApp
				err = gyaml.Unmarshal(appfileData, &workflowapp)
				if err != nil {
					log.Printf("[WARNING] Failed building workflowapp %s: %s", extra, err)
					return buildLaterFirst, buildLaterList, errors.New(fmt.Sprintf("Failed building %s: %s", extra, err))
					//continue
				}

				newName := workflowapp.Name
				newName = strings.ReplaceAll(newName, " ", "-")

				tags := []string{
					fmt.Sprintf("%s:%s_%s", baseDockerName, strings.ToLower(newName), workflowapp.AppVersion),
				}

				if len(allapps) == 0 {
					allapps, err = shuffle.GetAllWorkflowApps(ctx, 500)
					if err != nil {
						log.Printf("Failed getting apps to verify: %s", err)
						continue
					}
				}

				// Make an option to override existing apps?
				//Hash string `json:"hash" datastore:"hash" yaml:"hash"` // api.yaml+dockerfile+src/app.py for apps
				removeApps := []string{}
				skip := false
				for _, app := range allapps {
					if app.Name == workflowapp.Name && app.AppVersion == workflowapp.AppVersion {
						// FIXME: Check if there's a new APP_SDK as well.
						// Skip this check if app_sdk is new.
						if app.Hash == md5 && app.Hash != "" && !forceUpdate {
							skip = true
							break
						}

						//log.Printf("Overriding app %s:%s as it exists but has different hash.", app.Name, app.AppVersion)
						removeApps = append(removeApps, app.ID)
					}
				}

				if skip && !forceUpdate {
					continue
				}

				// Fixes (appends) authentication parameters if they're required
				if workflowapp.Authentication.Required {
					log.Printf("[INFO] Checking authentication fields and appending for %s!", workflowapp.Name)
					// FIXME:
					// Might require reflection into the python code to append the fields as well
					for index, action := range workflowapp.Actions {
						if action.AuthNotRequired {
							log.Printf("Skipping auth setup: %s", action.Name)
							continue
						}

						// 1. Check if authentication params exists at all
						// 2. Check if they're present in the action
						// 3. Add them IF they DONT exist
						// 4. Fix python code with reflection (FIXME)
						appendParams := []shuffle.WorkflowAppActionParameter{}
						for _, fieldname := range workflowapp.Authentication.Parameters {
							found := false
							for index, param := range action.Parameters {
								if param.Name == fieldname.Name {
									found = true

									action.Parameters[index].Configuration = true
									//log.Printf("Set config to true for field %s!", param.Name)
									break
								}
							}

							if !found {
								appendParams = append(appendParams, shuffle.WorkflowAppActionParameter{
									Name:          fieldname.Name,
									Description:   fieldname.Description,
									Example:       fieldname.Example,
									Required:      fieldname.Required,
									Configuration: true,
									Schema:        fieldname.Schema,
								})
							}
						}

						if len(appendParams) > 0 {
							log.Printf("[AUTH] Appending %d params to the START of %s", len(appendParams), action.Name)
							workflowapp.Actions[index].Parameters = append(appendParams, workflowapp.Actions[index].Parameters...)
						}

					}
				}

				err = checkWorkflowApp(workflowapp)
				if err != nil {
					log.Printf("%s for app %s:%s", err, workflowapp.Name, workflowapp.AppVersion)
					continue
				}

				if len(removeApps) > 0 {
					for _, item := range removeApps {
						log.Printf("[WARNING] Removing duplicate app: %s", item)
						err = DeleteKey(ctx, "workflowapp", item)
						if err != nil {
							log.Printf("[ERROR] Failed deleting duplicate %s: %s", item, err)
						}
					}
				}

				workflowapp.ID = uuid.NewV4().String()
				workflowapp.IsValid = true
				workflowapp.Verified = true
				workflowapp.Sharing = true
				workflowapp.Downloaded = true
				workflowapp.Hash = md5
				workflowapp.Public = true

				err = shuffle.SetWorkflowAppDatastore(ctx, workflowapp, workflowapp.ID)
				if err != nil {
					log.Printf("Failed setting workflowapp: %s", err)
					continue
				}

				/*
					err = increaseStatisticsField(ctx, "total_apps_created", workflowapp.ID, 1, "")
					if err != nil {
						log.Printf("Failed to increase total apps created stats: %s", err)
					}

					err = increaseStatisticsField(ctx, "total_apps_loaded", workflowapp.ID, 1, "")
					if err != nil {
						log.Printf("Failed to increase total apps loaded stats: %s", err)
					}
				*/

				//log.Printf("Added %s:%s to the database", workflowapp.Name, workflowapp.AppVersion)

				// ID  can be used to e.g. set a build status.
				buildLater := buildLaterStruct{
					Tags:  tags,
					Extra: extra,
					Id:    workflowapp.ID,
				}

				reservedFound := false
				for _, appname := range reservedNames {
					if strings.ToUpper(workflowapp.Name) == strings.ToUpper(appname) {
						buildLaterList = append(buildLaterList, buildLater)

						reservedFound = true
						break
					}
				}

				/// Only upload if successful and no errors
				if !reservedFound {
					buildLaterFirst = append(buildLaterFirst, buildLater)
				} else {
					log.Printf("[WARNING] Skipping build of %s to later", workflowapp.Name)
				}
			}
		}
	}

	if len(buildLaterFirst) == 0 && len(buildLaterList) == 0 {
		return buildLaterFirst, buildLaterList, err
	}

	// This is getting silly
	cacheKey := fmt.Sprintf("workflowapps-sorted-100")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-500")
	shuffle.DeleteCache(ctx, cacheKey)

	//log.Printf("BUILDLATERFIRST: %d, BUILDLATERLIST: %d", len(buildLaterFirst), len(buildLaterList))
	if len(extra) == 0 {
		log.Printf("[INFO] Starting build of %d containers (FIRST)", len(buildLaterFirst))
		for _, item := range buildLaterFirst {
			err = buildImageMemory(fs, item.Tags, item.Extra, true)
			if err != nil {
				log.Printf("Failed image build memory: %s", err)
			} else {
				if len(item.Tags) > 0 {
					log.Printf("[INFO] Successfully built image %s", item.Tags[0])

				} else {
					log.Printf("[INFO] Successfully built Docker image")
				}
			}
		}

		log.Printf("[INFO] Starting build of %d skipped docker images", len(buildLaterList))
		for _, item := range buildLaterList {
			err = buildImageMemory(fs, item.Tags, item.Extra, true)
			if err != nil {
				log.Printf("[INFO] Failed image build memory: %s", err)
			} else {
				if len(item.Tags) > 0 {
					log.Printf("[INFO] Successfully built image %s", item.Tags[0])
				} else {
					log.Printf("[INFO] Successfully built Docker image")
				}
			}
		}
	}

	return buildLaterFirst, buildLaterList, err
}

func setNewWorkflowApp(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	// Just need to be logged in
	// FIXME - should have some permissions?
	_, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in set new app: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Error with body read: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	var workflowapp shuffle.WorkflowApp
	err = json.Unmarshal(body, &workflowapp)
	if err != nil {
		log.Printf("Failed unmarshaling: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	ctx := context.Background()
	allapps, err := shuffle.GetAllWorkflowApps(ctx, 500)
	if err != nil {
		log.Printf("Failed getting apps to verify: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	appfound := false
	for _, app := range allapps {
		if app.Name == workflowapp.Name && app.AppVersion == workflowapp.AppVersion {
			log.Printf("App upload for %s:%s already exists.", app.Name, app.AppVersion)
			appfound = true
			break
		}
	}

	if appfound {
		log.Printf("App %s:%s already exists. Bump the version.", workflowapp.Name, workflowapp.AppVersion)
		resp.WriteHeader(409)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "App %s:%s already exists."}`, workflowapp.Name, workflowapp.AppVersion)))
		return
	}

	err = checkWorkflowApp(workflowapp)
	if err != nil {
		log.Printf("%s for app %s:%s", err, workflowapp.Name, workflowapp.AppVersion)
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s for app %s:%s"}`, err, workflowapp.Name, workflowapp.AppVersion)))
		return
	}

	//if workflowapp.Environment == "" {
	//	workflowapp.Environment = baseEnvironment
	//}

	workflowapp.ID = uuid.NewV4().String()
	workflowapp.IsValid = true
	workflowapp.Generated = false
	workflowapp.Activated = true

	err = shuffle.SetWorkflowAppDatastore(ctx, workflowapp, workflowapp.ID)
	if err != nil {
		log.Printf("Failed setting workflowapp: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	} else {
		log.Printf("Added %s:%s to the database", workflowapp.Name, workflowapp.AppVersion)
	}

	cacheKey := fmt.Sprintf("workflowapps-sorted-100")
	shuffle.DeleteCache(ctx, cacheKey)
	cacheKey = fmt.Sprintf("workflowapps-sorted-500")
	shuffle.DeleteCache(ctx, cacheKey)

	resp.WriteHeader(200)
	resp.Write([]byte(fmt.Sprintf(`{"success": true}`)))
}

func getWorkflowExecutions(resp http.ResponseWriter, request *http.Request) {
	cors := handleCors(resp, request)
	if cors {
		return
	}

	user, err := shuffle.HandleApiAuthentication(resp, request)
	if err != nil {
		log.Printf("Api authentication failed in getting specific workflow: %s", err)
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

	if len(fileId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID when getting workflow executions is not valid"}`))
		return
	}

	ctx := context.Background()
	workflow, err := shuffle.GetWorkflow(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting the workflow %s locally (get executions): %s", fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// FIXME - have a check for org etc too..
	if user.Id != workflow.Owner {
		log.Printf("Wrong user (%s) for workflow %s (get execution)", user.Username, workflow.ID)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	// Query for the specifci workflowId
	maxAmount := 30
	q := datastore.NewQuery("workflowexecution").Filter("workflow_id =", fileId).Order("-started_at").Limit(maxAmount)
	var workflowExecutions []shuffle.WorkflowExecution
	_, err = dbclient.GetAll(ctx, q, &workflowExecutions)
	if err != nil {

		if strings.Contains(fmt.Sprintf("%s", err), "ResourceExhausted") {
			q = datastore.NewQuery("workflowexecution").Filter("workflow_id =", fileId).Order("-started_at").Limit(1)
			/*
				_, err = dbclient.GetAll(ctx, q, &workflowExecutions)
				if err != nil {
					log.Printf("Error getting workflowexec (2): %s", err)
					resp.WriteHeader(401)
					resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting all workflowexecutions for %s"}`, fileId)))
					return
				}
			*/

			cursorStr := ""
			for {
				it := dbclient.Run(ctx, q)
				//_, err = it.Next(&app)
				for {
					var workflowExecution shuffle.WorkflowExecution
					_, err := it.Next(&workflowExecution)
					if err != nil {
						break
					}

					workflowExecutions = append(workflowExecutions, workflowExecution)
				}

				//log.Printf("Len: %d", len(workflowExecutions))
				if len(workflowExecutions) > maxAmount {
					break
				}

				nextCursor, err := it.Cursor()
				if err != iterator.Done && err != nil {
					if strings.Contains(fmt.Sprintf("%s", err), "ResourceExhausted") {
						//log.Printf("NEXT!")
						nextStr := fmt.Sprintf("%s", nextCursor)
						if cursorStr == nextStr {
							break
						}

						cursorStr = nextStr

						continue
					} else {
						log.Printf("BREAK: %s", err)
						break
					}
				}

				if err != nil {
					if strings.Contains(fmt.Sprintf("%s", err), "ResourceExhausted") {
						log.Printf("[WARNING] Cursorerror in app grab WARNING: %s", err)
					} else {
						log.Printf("[ERROR] Cursorerror in app grab: %s", err)
						break
					}
				} else {
					//log.Printf("NEXTCURSOR: %s", nextCursor)
					nextStr := fmt.Sprintf("%s", nextCursor)
					if cursorStr == nextStr {
						break
					}

					cursorStr = nextStr
					q = q.Start(nextCursor)
					//cursorStr = nextCursor
					//break
				}
			}
		} else {
			log.Printf("Error getting workflowexec: %s", err)
			resp.WriteHeader(401)
			resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed getting all workflowexecutions for %s"}`, fileId)))
			return
		}
	}

	if len(workflowExecutions) == 0 {
		resp.WriteHeader(200)
		resp.Write([]byte("[]"))
		return
	}

	newjson, err := json.Marshal(workflowExecutions)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "Failed unpacking workflow executions"}`)))
		return
	}

	resp.WriteHeader(200)
	resp.Write(newjson)
}

func getAllSchedules(ctx context.Context, orgId string) ([]ScheduleOld, error) {
	var schedules []ScheduleOld

	q := datastore.NewQuery("schedules").Filter("org = ", orgId)
	if orgId == "ALL" {
		q = datastore.NewQuery("schedules")
	}

	_, err := dbclient.GetAll(ctx, q, &schedules)
	if err != nil {
		return []ScheduleOld{}, err
	}

	return schedules, nil
}

//FIXME: Add cursor
//func shuffle.GetAllWorkflowApps(ctx context.Context, maxLen int) ([]shuffle.WorkflowApp, error) {
//	var apps []WorkflowApp
//	query := datastore.NewQuery("workflowapp").Order("-edited").Limit(10)
//	//query := datastore.NewQuery("workflowapp").Order("-edited").Limit(40)
//
//	cacheKey := fmt.Sprintf("workflowapps-sorted-%d", maxLen)
//	if value, found := requestCache.Get(cacheKey); found {
//		parsedValue := value.(*[]WorkflowApp)
//		log.Printf("[INFO] Returning %d apps from cache", len(*parsedValue))
//		return *parsedValue, nil
//	}
//
//	cursorStr := ""
//
//	// NOT BEING UPDATED
//	// FIXME: Update the app with the correct actions. HOW DOES THIS WORK??
//	// Seems like only actions are wrong. Could get the app individually.
//	// Guessing it's a memory issue.
//	//Actions        []WorkflowAppAction `json:"actions" yaml:"actions" required:true datastore:"actions,noindex"`
//	//errors.New(nil)
//	var err error
//	for {
//		it := dbclient.Run(ctx, query)
//		//_, err = it.Next(&app)
//		for {
//			var app WorkflowApp
//			_, err := it.Next(&app)
//			if err != nil {
//				break
//			}
//
//			if app.Name == "Shuffle Subflow" {
//				continue
//			}
//
//			found := false
//			//log.Printf("ACTIONS: %d - %s", len(app.Actions), app.Name)
//			for _, innerapp := range apps {
//				if innerapp.Name == app.Name {
//					found = true
//					break
//				}
//			}
//
//			if !found {
//				apps = append(apps, app)
//			}
//		}
//
//		if err != iterator.Done {
//			//log.Printf("[INFO] Failed fetching results: %v", err)
//			//break
//		}
//
//		// Get the cursor for the next page of results.
//		nextCursor, err := it.Cursor()
//		if err != nil {
//			log.Printf("Cursorerror: %s", err)
//			break
//		} else {
//			//log.Printf("NEXTCURSOR: %s", nextCursor)
//			nextStr := fmt.Sprintf("%s", nextCursor)
//			if cursorStr == nextStr {
//				break
//			}
//
//			cursorStr = nextStr
//			query = query.Start(nextCursor)
//			//cursorStr = nextCursor
//			//break
//		}
//
//		if len(apps) > maxLen {
//			break
//		}
//	}
//
//	if len(apps) > 20 {
//		log.Printf("[INFO] Setting %d apps in cache", len(apps))
//		requestCache.Set(cacheKey, &apps, cache.DefaultExpiration)
//	}
//
//	//var allworkflowapps []WorkflowApp
//	//_, err := dbclient.GetAll(ctx, query, &allworkflowapps)
//	//if err != nil {
//	//	if strings.Contains(fmt.Sprintf("%s", err), "ResourceExhausted") {
//	//		//datastore.NewQuery("workflowapp").Limit(30).Order("-edited")
//	//		query = datastore.NewQuery("workflowapp").Order("-edited").Limit(25)
//	//		//q := q.Limit(25)
//	//		_, err := dbclient.GetAll(ctx, query, &allworkflowapps)
//	//		if err != nil {
//	//			return []WorkflowApp{}, err
//	//		}
//	//	} else {
//	//		return []WorkflowApp{}, err
//	//	}
//	//}
//
//	return apps, nil
//}

//func shuffle.GetAllWorkflowAppAuth(ctx context.Context, OrgId string) ([]shuffle.AppAuthenticationStorage, error) {
//	var allworkflowapps []AppAuthenticationStorage
//	q := datastore.NewQuery("workflowappauth").Filter("org_id = ", OrgId)
//
//	_, err := dbclient.GetAll(ctx, q, &allworkflowapps)
//	if err != nil {
//		return []AppAuthenticationStorage{}, err
//	}
//
//	return allworkflowapps, nil
//}
//
//func getWorkflowAppAuthDatastore(ctx context.Context, id string) (*AppAuthenticationStorage, error) {
//
//	key := datastore.NameKey("workflowappauth", id, nil)
//	appAuth := &AppAuthenticationStorage{}
//	// New struct, to not add body, author etc
//	if err := dbclient.Get(ctx, key, appAuth); err != nil {
//		return &AppAuthenticationStorage{}, err
//	}
//
//	return appAuth, nil
//}
//
//func shuffle.SetWorkflowAppAuthDatastore(ctx context.Context, workflowappauth AppAuthenticationStorage, id string) error {
//	timeNow := int64(time.Now().Unix())
//	if workflowappauth.Created == 0 {
//		workflowappauth.Created = timeNow
//	}
//
//	workflowappauth.Edited = timeNow
//
//	key := datastore.NameKey("workflowappauth", id, nil)
//
//	// New struct, to not add body, author etc
//	if _, err := dbclient.Put(ctx, key, &workflowappauth); err != nil {
//		log.Printf("Error adding workflow app auth: %s", err)
//		return err
//	}
//
//	return nil
//}
//
//// Hmm, so I guess this should use uuid :(
//// Consistency PLX
//func SetWorkflowAppDatastore(ctx context.Context, workflowapp WorkflowApp, id string) error {
//	timeNow := int64(time.Now().Unix())
//	if workflowapp.Created == 0 {
//		workflowapp.Created = timeNow
//	}
//
//	workflowapp.Edited = timeNow
//	key := datastore.NameKey("workflowapp", id, nil)
//
//	// New struct, to not add body, author etc
//	if _, err := dbclient.Put(ctx, key, &workflowapp); err != nil {
//		log.Printf("Error adding workflow app: %s", err)
//		return err
//	}
//
//	return nil
//}

// Starts a new webhook
func handleStopHook(resp http.ResponseWriter, request *http.Request) {
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
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID when stopping hook is not valid"}`))
		return
	}

	ctx := context.Background()
	hook, err := getHook(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting hook %s (stop): %s", fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Id != hook.Owner {
		log.Printf("Wrong user (%s) for workflow %s", user.Username, hook.Id)
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
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, message)))
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
	// FIXME
	err = removeWebhookFunction(ctx, fileId)
	if err != nil {
		log.Printf("Container stop issue for %s-%s: %s", image, fileId, err)
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true, "reason": "Stopped webhook"}`))
}

func removeWebhookFunction(ctx context.Context, hookid string) error {
	service, err := cloudfunctions.NewService(ctx)
	if err != nil {
		return err
	}

	// ProjectsLocationsListCall
	projectsLocationsFunctionsService := cloudfunctions.NewProjectsLocationsFunctionsService(service)
	location := fmt.Sprintf("projects/%s/locations/%s", gceProject, defaultLocation)
	functionName := fmt.Sprintf("%s/functions/webhook_%s", location, hookid)

	deleteCall := projectsLocationsFunctionsService.Delete(functionName)
	resp, err := deleteCall.Do()
	if err != nil {
		log.Printf("Failed to delete %s from %s: %s", hookid, defaultLocation, err)
		return err
	} else {
		log.Printf("Successfully deleted %s from %s", hookid, defaultLocation)
	}

	_ = resp
	return nil
}

func handleStartHook(resp http.ResponseWriter, request *http.Request) {
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

	if len(fileId) != 36 {
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false, "reason": "Workflow ID when starting hook is not valid"}`))
		return
	}

	ctx := context.Background()
	hook, err := getHook(ctx, fileId)
	if err != nil {
		log.Printf("Failed getting hook %s (start): %s", fileId, err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	if user.Id != hook.Owner {
		log.Printf("Wrong user (%s) for workflow %s", user.Username, hook.Id)
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
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, message)))
		return
	}

	environmentVariables := map[string]string{
		"FUNCTION_APIKEY": user.ApiKey,
		"CALLBACKURL":     syncUrl,
		"HOOKID":          fileId,
	}

	applocation := fmt.Sprintf("gs://%s/triggers/webhook.zip", bucketName)
	hookname := fmt.Sprintf("webhook_%s", fileId)
	err = deployWebhookFunction(ctx, hookname, "europe-west2", applocation, environmentVariables)
	if err != nil {
		resp.WriteHeader(401)
		resp.Write([]byte(fmt.Sprintf(`{"success": false, "reason": "%s"}`, err)))
		return
	}

	hook.Status = "running"
	hook.Running = true
	err = setHook(ctx, *hook)
	if err != nil {
		log.Printf("Failed setting hook: %s", err)
		resp.WriteHeader(401)
		resp.Write([]byte(`{"success": false}`))
		return
	}

	log.Printf("Starting function %s?", fileId)
	resp.WriteHeader(200)
	resp.Write([]byte(`{"success": true, "reason": "Started webhook"}`))
	return
}

func removeOutlookTriggerFunction(ctx context.Context, triggerId string) error {
	service, err := cloudfunctions.NewService(ctx)
	if err != nil {
		return err
	}

	// ProjectsLocationsListCall
	projectsLocationsFunctionsService := cloudfunctions.NewProjectsLocationsFunctionsService(service)
	location := fmt.Sprintf("projects/%s/locations/%s", gceProject, defaultLocation)
	functionName := fmt.Sprintf("%s/functions/outlooktrigger_%s", location, triggerId)

	deleteCall := projectsLocationsFunctionsService.Delete(functionName)
	resp, err := deleteCall.Do()
	if err != nil {
		log.Printf("Failed to delete %s from %s: %s", triggerId, defaultLocation, err)
		return err
	} else {
		log.Printf("Successfully deleted %s from %s", triggerId, defaultLocation)
	}

	_ = resp
	return nil
}

func handleUserInput(trigger shuffle.Trigger, organizationId string, workflowId string, referenceExecution string) error {
	// E.g. check email
	sms := ""
	email := ""
	triggerType := ""
	triggerInformation := ""
	for _, item := range trigger.Parameters {
		if item.Name == "alertinfo" {
			triggerInformation = item.Value
		} else if item.Name == "type" {
			triggerType = item.Value
		} else if item.Name == "email" {
			email = item.Value
		} else if item.Name == "sms" {
			sms = item.Value
		}
	}

	if len(triggerType) == 0 {
		log.Printf("No type specified for user input node")
		return errors.New("No type specified for user input node")
	}

	// FIXME: This is not the right time to send them, BUT it's well served for testing. Save -> send email / sms
	ctx := context.Background()
	startNode := trigger.ID
	if strings.Contains(triggerType, "email") {
		action := shuffle.CloudSyncJob{
			Type:          "user_input",
			Action:        "send_email",
			OrgId:         organizationId,
			PrimaryItemId: workflowId,
			SecondaryItem: startNode,
			ThirdItem:     triggerInformation,
			FourthItem:    email,
			FifthItem:     referenceExecution,
		}

		org, err := shuffle.GetOrg(ctx, organizationId)
		if err != nil {
			log.Printf("Failed email send to cloud (1): %s", err)
			return err
		}

		err = executeCloudAction(action, org.SyncConfig.Apikey)
		if err != nil {
			log.Printf("Failed email send to cloud (2): %s", err)
			return err
		}

		log.Printf("Should send email to %s during execution.", email)
	}
	if strings.Contains(triggerType, "sms") {
		action := shuffle.CloudSyncJob{
			Type:          "user_input",
			Action:        "send_sms",
			OrgId:         organizationId,
			PrimaryItemId: workflowId,
			SecondaryItem: startNode,
			ThirdItem:     triggerInformation,
			FourthItem:    sms,
			FifthItem:     referenceExecution,
		}

		org, err := shuffle.GetOrg(ctx, organizationId)
		if err != nil {
			log.Printf("Failed sms send to cloud (3): %s", err)
			return err
		}

		err = executeCloudAction(action, org.SyncConfig.Apikey)
		if err != nil {
			log.Printf("Failed sms send to cloud (4): %s", err)
			return err
		}

		log.Printf("Should send SMS to %s during execution.", sms)
	}

	return nil
}

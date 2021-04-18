import React, { useEffect} from 'react';
import { useInterval } from 'react-powerhooks';
import { makeStyles } from '@material-ui/core/styles';

import {Avatar, Grid, Paper, Tooltip, Divider, Button, TextField, FormControl, IconButton, Menu, MenuItem, FormControlLabel, Chip, Switch, Typography, Zoom, CircularProgress, Dialog, DialogTitle, DialogActions, DialogContent} from '@material-ui/core';
import {FileCopy as FileCopyIcon, Delete as DeleteIcon, BubbleChart as BubbleChartIcon, Restore as RestoreIcon, Cached as CachedIcon, GetApp as GetAppIcon, Apps as AppsIcon, Edit as EditIcon, MoreVert as MoreVertIcon, PlayArrow as PlayArrowIcon, Add as AddIcon, Publish as PublishIcon, CloudUpload as CloudUploadIcon, CloudDownload as CloudDownloadIcon} from '@material-ui/icons';
//import {Search as SearchIcon, ArrowUpward as ArrowUpwardIcon, Visibility as VisibilityIcon, Done as DoneIcon, Close as CloseIcon, Error as ErrorIcon, FindReplace as FindreplaceIcon, ArrowLeft as ArrowLeftIcon, Cached as CachedIcon, DirectionsRun as DirectionsRunIcon, Add as AddIcon, Polymer as PolymerIcon, FormatListNumbered as FormatListNumberedIcon, Create as CreateIcon, PlayArrow as PlayArrowIcon, AspectRatio as AspectRatioIcon, MoreVert as MoreVertIcon, Apps as AppsIcon, Schedule as ScheduleIcon, FavoriteBorder as FavoriteBorderIcon, Pause as PauseIcon, Delete as DeleteIcon, AddCircleOutline as AddCircleOutlineIcon, Save as SaveIcon, KeyboardArrowLeft as KeyboardArrowLeftIcon, KeyboardArrowRight as KeyboardArrowRightIcon, ArrowBack as ArrowBackIcon, Settings as SettingsIcon, LockOpen as LockOpenIcon, ExpandMore as ExpandMoreIcon, VpnKey as VpnKeyIcon} from '@material-ui/icons';

import {DataGrid, GridToolbarContainer, GridDensitySelector, GridToolbar} from '@material-ui/data-grid';

//import JSONPretty from 'react-json-pretty';
//import JSONPrettyMon from 'react-json-pretty/dist/monikai'
import ReactJson from 'react-json-view'
import Dropzone from '../components/Dropzone';

import {Link} from 'react-router-dom';
import { useAlert } from "react-alert";
import ChipInput from 'material-ui-chip-input'
import uuid from "uuid"
import CytoscapeWrapper from '../components/RenderCytoscape'

//import mobileImage from '../assets/img/mobile.svg';
//import bagImage from '../assets/img/bag.svg';
//import bookImage from '../assets/img/book.svg';

const inputColor = "#383B40"
const surfaceColor = "#27292D"

const flexContainerStyle = {
	display: "flex",
	flexDirection: "row",
	justifyContent: "left",
	alignContent: "space-between",
}

const flexBoxStyle = {
	height: 125,
	borderRadius: 4,
	boxSizing: "border-box",
	letterSpacing: "0.4px",
	color: "#D6791E",
	margin: 10, 
	flex: 1, 
}

const useStyles = makeStyles((theme) => ({
	root: {
		border: 0,
		'& .MuiDataGrid-columnsContainer': {
		backgroundColor: theme.palette.type === 'light' ? '#fafafa' : '#1d1d1d',
		},
		'& .MuiDataGrid-iconSeparator': {
		display: 'none',
		},
		'& .MuiDataGrid-colCell, .MuiDataGrid-cell': {
		borderRight: `1px solid ${
			theme.palette.type === 'light' ? 'white' : '#303030'
		}`,
		},
		'& .MuiDataGrid-columnsContainer, .MuiDataGrid-cell': {
		borderBottom: `1px solid ${
			theme.palette.type === 'light' ? '#f0f0f0' : '#303030'
		}`,
		},
		'& .MuiDataGrid-cell': {
		color:
			theme.palette.type === 'light'
			? 'white'
			: 'rgba(255,255,255,0.65)',
		},
		'& .MuiPaginationItem-root, .MuiTablePagination-actions, .MuiTablePagination-caption': {
		borderRadius: 0,
		color: "white",
		},
	},
}));

//const activeWorkflowStyle = {backgroundColor: "#FFF5EE"}
//const notificationStyle = {backgroundColor: "#E5F9FF"}
//const activeWorkflowStyle = {backgroundColor: "#3d3f43"}
const availableWorkflowStyle = {backgroundColor: "#3d3f43"}
const notificationStyle = {backgroundColor: "#3d3f43"}
const activeWorkflowStyle = {backgroundColor: "#3d3f43"}

const fontSize_16 = {fontSize: "16px",}
const counterStyle = {fontSize: "36px",fontWeight:"bold"}
const blockRightStyle = {textAlign: "right",padding: "20px 20px 0px 0px",width:"100%"}

const chipStyle = {
	backgroundColor: "#3d3f43", height: 30, marginRight: 5, paddingLeft: 5, paddingRight: 5, height: 28, cursor: "pointer", borderColor: "#3d3f43", color: "white",
}

const flexContentStyle = {
	display: "flex", 
	flexDirection: "row"
}

const iconStyle = {
	width: "75px",
	height: "75px",
	padding: "20px"
}

export const validateJson = (showResult) => {
	//showResult = showResult.split(" None").join(" \"None\"")
	showResult = showResult.split(" False").join(" false")
	showResult = showResult.split(" True").join(" true")

	var jsonvalid = true
	try {
		const tmp = String(JSON.parse(showResult))
		if (!showResult.includes("{") && !showResult.includes("[")) {
			jsonvalid = false
		}
	} catch (e) {
		showResult = showResult.split("\'").join("\"")

		try {
			const tmp = String(JSON.parse(showResult))
			if (!showResult.includes("{") && !showResult.includes("[")) {
				jsonvalid = false
			}
		} catch (e) {
			jsonvalid = false
		}
	}

	const result = jsonvalid ? JSON.parse(showResult) : showResult
	//console.log("VALID: ", jsonvalid, result)
	return {
		"valid": jsonvalid, 
		"result": result,
	}
}

const Workflows = (props) => {
  const { globalUrl, isLoggedIn, isLoaded, removeCookie, cookies, userdata} = props;
	document.title = "Shuffle - Workflows"

	const alert = useAlert()
	const classes = useStyles();

	var upload = ""
	const [file, setFile] = React.useState("");

	const [workflows, setWorkflows] = React.useState([]);
	const [filteredWorkflows, setFilteredWorkflows] = React.useState([]);
	const [selectedWorkflow, setSelectedWorkflow] = React.useState({});
	const [selectedExecution, setSelectedExecution] = React.useState({});
	const [workflowExecutions, setWorkflowExecutions] = React.useState([]);
	const [firstrequest, setFirstrequest] = React.useState(true)
	const [workflowDone, setWorkflowDone] = React.useState(false)
	const [, setTrackingId] = React.useState("")
	const [selectedWorkflowId, setSelectedWorkflowId] = React.useState("")

	const [collapseJson, setCollapseJson] = React.useState(false)
	const [field1, setField1] = React.useState("")
	const [field2, setField2] = React.useState("")
	const [downloadUrl, setDownloadUrl] = React.useState("https://github.com/frikky/shuffle-workflows")
	const [downloadBranch, setDownloadBranch] = React.useState("master")
	const [loadWorkflowsModalOpen, setLoadWorkflowsModalOpen] = React.useState(false)

	const [modalOpen, setModalOpen] = React.useState(false);
	const [newWorkflowName, setNewWorkflowName] = React.useState("");
	const [newWorkflowDescription, setNewWorkflowDescription] = React.useState("");
	const [newWorkflowTags, setNewWorkflowTags] = React.useState([]);
	const [update, setUpdate] = React.useState("test");
	const [deleteModalOpen, setDeleteModalOpen] = React.useState(false);
	const [editingWorkflow, setEditingWorkflow] = React.useState({})
	const [executionLoading, setExecutionLoading] = React.useState(false)
	const [importLoading, setImportLoading] = React.useState(false)
	const [isDropzone, setIsDropzone] = React.useState(false);
	const [view, setView] = React.useState("grid")
	const [filters, setFilters] = React.useState([])
	const isCloud = window.location.host === "localhost:3002" || window.location.host === "shuffler.io" 

	const findWorkflow = (filters) => {
		if (filters.length === 0) {
			setFilteredWorkflows(workflows)
			return
		}

		var newWorkflows = []
		for (var workflowKey in workflows) {
			const curWorkflow = workflows[workflowKey]

			var found = [false]
			if (curWorkflow.tags === undefined || curWorkflow.tags === null) {
				found = filters.map(filter => curWorkflow.name.toLowerCase().includes(filter))
			} else {
				found = filters.map(filter => curWorkflow.name.toLowerCase().includes(filter.toLowerCase()) || curWorkflow.tags.includes(filter))
			}
			//console.log("FOUND: ", found)
			//if (found) {
			if (found.every(v => v === true)) {
				newWorkflows.push(curWorkflow)
				continue
			}
		}

		if (newWorkflows.length !== workflows.length) {
			setFilteredWorkflows(newWorkflows)
		}
	}

	const addFilter = (data) => {
		if (data === null || data === undefined) {
			return
		}

		if (data.includes("<") && data.includes(">")) {
			return
		}

		if (filters.includes(data)) {
			return
		}

		filters.push(data.toLowerCase())
		setFilters(filters)

		findWorkflow(filters)
	}

	const removeFilter = (index) => {
		var newfilters = filters

		if (index < 0) {
			console.log("Can't handle index: ", index)
			return
		}


		//console.log("Removing filter index", index)
		newfilters.splice(index, 1)
		//console.log("FILTER LENGTH: ", filters.length)

		if (newfilters.length === 0) {
			newfilters = []
			setFilters(newfilters)
		} else {
			setFilters(newfilters)
		}
		//console.log("FILTERS: ", newfilters)

		findWorkflow(newfilters) 
	}

	const deleteModal = deleteModalOpen ? 
		<Dialog
			open={deleteModalOpen}
			onClose={() => {
				setDeleteModalOpen(false)
				setSelectedWorkflowId("")
			}}
			PaperProps={{
				style: {
					backgroundColor: surfaceColor,
					color: "white",
					minWidth: 500,
				},
			}}
		>
			<DialogTitle>
				<div style={{textAlign: "center", color: "rgba(255,255,255,0.9)"}}>
					Are you sure? <div/>Other workflows relying on this one may stop working
				</div>
			</DialogTitle>
			<DialogContent style={{color: "rgba(255,255,255,0.65)", textAlign: "center"}}>
				<Button style={{}} onClick={() => {
					console.log("Editing: ", editingWorkflow)
					if (selectedWorkflowId) {
						deleteWorkflow(selectedWorkflowId)		
						getAvailableWorkflows() 
					}
					setDeleteModalOpen(false)
				}} color="primary">
					Yes
				</Button>
				<Button variant="outlined" style={{}} onClick={() => {setDeleteModalOpen(false)}} color="primary">
					No
				</Button>
			</DialogContent>
			
		</Dialog>
	: null

	const uploadFile = (e) => {
		const isDropzone = e.dataTransfer === undefined ? false : e.dataTransfer.files.length > 0;
		const files = isDropzone ? e.dataTransfer.files : e.target.files;
		
    const reader = new FileReader();
		alert.info("Starting upload. Please wait while we validate the workflows")

		try {
			reader.addEventListener('load', (e) => {
				var data = e.target.result;
				setIsDropzone(false)
				try {
					data = JSON.parse(reader.result)
				} catch (e) {
					alert.error("Invalid JSON: "+e)
					return
				}

				// Initialize the workflow itself
				const ret = setNewWorkflow(data.name, data.description, data.tags, {}, false)
				.then((response) => {
					if (response !== undefined) {
						// SET THE FULL THING
						data.id = response.id

						// Actually create it
						const ret = setNewWorkflow(data.name, data.description, data.tags, data, false)
						.then((response) => {
							if (response !== undefined) {
								alert.success("Successfully imported "+data.name)
							}
						})
					}
				})
				.catch(error => {
					alert.error("Import error: "+error.toString())
				});
			})
		} catch (e) {
			console.log("Error in dropzone: ", e)
		}

		reader.readAsText(files[0]);
  }

	useEffect(() => {
		if (isDropzone) {
			//redirectOpenApi();
			setIsDropzone(false);
		}
  }, [isDropzone]);

	const getAvailableWorkflows = () => {
		fetch(globalUrl+"/api/v1/workflows", {
    	  method: 'GET',
				headers: {
					'Content-Type': 'application/json',
					'Accept': 'application/json',
				},
	  		credentials: "include",
    })
		.then((response) => {
			if (response.status !== 200) {
				console.log("Status not 200 for workflows :O!: ", response.status)

				if (isCloud) {
					window.location.pathname = "/login"
				}

				alert.info("Failed getting workflows.")
				setWorkflowDone(true)

				return 
			}
			return response.json()
		})
    .then((responseJson) => {
			setSelectedExecution({})
			setWorkflowExecutions([])
			//console.log(responseJson)

			if (responseJson !== undefined) {
				setWorkflows(responseJson)
				setFilteredWorkflows(responseJson)
				setWorkflowDone(true)
			} else {
				if (isLoggedIn) {
					alert.error("An error occurred while loading workflows")
				}

				return
			}

			if (responseJson.length > 0){
				//setSelectedWorkflow(responseJson[0])
				//getWorkflowExecution(responseJson[0].id)
			}
    	})
		.catch(error => {
			alert.error(error.toString())
		});
	}

	useEffect(() => {
		if (workflows.length <= 0 && firstrequest) {
			setFirstrequest(false)
			getAvailableWorkflows()
		}
	})

	const viewStyle = {
		color: "#ffffff",
		width: "100%",
		display: "flex",
		minWidth: 1024,
		maxWidth: 1024,
		margin: "auto",
		/*maxHeight: "90vh",*/
	}

	const emptyWorkflowStyle = {
		paddingTop: "200px", 
		width: 1024,
		margin: "auto",
	}

	const boxStyle = {
		padding: "20px 20px 20px 20px",
		width: "100%",
		height: "250px",
		color: "white",
		backgroundColor: surfaceColor,
		display: "flex", 
		flexDirection: "column",
	}


	const scrollStyle = {
		marginTop: "10px",
		overflow: "scroll",
		height: "90%",
		overflowX: "hidden",
		overflowY: "auto",
	}

	const paperAppContainer = {
		display: "flex",
		flexWrap: 'wrap',
		alignContent: "space-between",
	}

	const paperAppStyle = {
		minHeight: 130,
		width: "100%",
		color: "white",
		backgroundColor: surfaceColor,
		padding: "12px 12px 0px 15px", 
		borderRadius: 5, 
		display: "flex",
		boxSizing: "border-box",
		position: "relative",
	}

	const gridContainer = {
		height: "auto",
		color: "white",
		margin: "10px",
		backgroundColor: surfaceColor,
	}

	const workflowActionStyle = {
		display: "flex", 
		width: 160, 
		height: 44,
		justifyContent: "space-between", 
	}

	const executeWorkflow = (id) => {
		alert.show("Executing workflow "+id)
		setTrackingId(id)
		fetch(globalUrl+"/api/v1/workflows/"+id+"/execute", {
    	  method: 'GET',
				headers: {
					'Content-Type': 'application/json',
					'Accept': 'application/json',
				},
	  			credentials: "include",
    		})
		.then((response) => {
			if (response.status !== 200) {
				console.log("Status not 200 for WORKFLOW EXECUTION :O!")
			} 

			return response.json()
		})
    	.then((responseJson) => {
			if (!responseJson.success) {
				alert.error(responseJson.reason)
			}
		})
		.catch(error => {
			alert.error(error.toString())
		});
	}

	function sleep (time) {
		return new Promise((resolve) => setTimeout(resolve, time));
	}

	const exportAllWorkflows = () => {
		for (var key in workflows) {
			exportWorkflow(workflows[key])
		}
	}

	const sanitizeWorkflow = (data) => {
		data["owner"] = ""
		console.log("Sanitize start: ", data)
		if (data.triggers !== null && data.triggers !== undefined) {
			for (var key in data.triggers) {
				const trigger = data.triggers[key]
				if (trigger.app_name === "Shuffle Workflow") {
					if (trigger.parameters.length > 2) {
						trigger.parameters[2].value = ""
					}
				} 
				
				if (trigger.status == "running") {
					trigger.status = "stopped"
				}

				const newId = uuid.v4()
				for (var branchkey in data.branches) {
					const branch = data.branches[branchkey]
					if (branch.source_id === trigger.id) {
						//console.log("CHANGING SOURCE ID")
						branch.source_id = newId
					} 

					if (branch.destination_id === trigger.id) {
						//console.log("CHANGING DESTINATION ID")
						branch.destination_id = newId
					}
				}

				trigger.environment = isCloud ? "cloud" : "Shuffle"
				trigger.id = newId
			}
		}

		if (data.actions !== null && data.actions !== undefined) {
			for (var key in data.actions) {
				data.actions[key].authentication_id = ""

				for (var subkey in data.actions[key].parameters) {
					const param = data.actions[key].parameters[subkey]
					if (param.name.includes("key") || param.name.includes("user") || param.name.includes("pass") || param.name.includes("api") || param.name.includes("auth") || param.name.includes("secret")) {
						// FIXME: This may be a vuln if api-keys are generated that start with $
						if (param.value.startsWith("$")) {
							console.log("Skipping field, as it's referencing a variable")
						} else {
							param.value = ""
							param.is_valid = false
						}
					}
				}

				const newId = uuid.v4()
				for (var branchkey in data.branches) {
					const branch = data.branches[branchkey]
					if (branch.source_id === data.actions[key].id) {
						//console.log("CHANGING SOURCE ID IN ACTION")
						branch.source_id = newId
					} 

					if (branch.destination_id === data.actions[key].id) {
						//console.log("CHANGING DESTINATION ID IN ACTION")
						branch.destination_id = newId
					}
				}

				//data.actions[key].environment = isCloud ? "cloud" : "Shuffle"
				data.actions[key].environment = ""
				data.actions[key].id = newId
			}
		}

		if (data.workflow_variables !== null && data.workflow_variables !== undefined) {
			for (var key in data.workflow_variables) {
				const param = data.workflow_variables[key]
				if (param.name.includes("key") || param.name.includes("user") || param.name.includes("pass") || param.name.includes("api") || param.name.includes("auth") || param.name.includes("secret")) {
					param.value = ""
					param.is_valid = false
				}
			}
		}

		//console.log(data)
		//return

		data["org"] = []
		data["org_id"] = ""
		data["execution_org"] = {}

		// These are backwards.. True = saved before. Very confuse.
		data["previously_saved"] = false
		data["first_save"] = false
		console.log("Sanitize end: ", data)

		return data
	}

	const exportWorkflow = (data) => {
		let exportFileDefaultName = data.name+'.json';
		data = sanitizeWorkflow(data)	

		//console.log("EXPORT: ", data)
		//return

		let dataStr = JSON.stringify(data)
		let dataUri = 'data:application/json;charset=utf-8,'+ encodeURIComponent(dataStr);
		let linkElement = document.createElement('a');
		linkElement.setAttribute('href', dataUri);
		linkElement.setAttribute('download', exportFileDefaultName);
		linkElement.click();
	}

	const publishWorkflow = (data) => {
		data = JSON.parse(JSON.stringify(data))
		data = sanitizeWorkflow(data) 
		alert.info("Sanitizing and publishing "+data.name)

		// This ALWAYS talks to Shuffle cloud
		fetch(globalUrl+"/api/v1/workflows/"+data.id+"/publish", {
    	  method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'Accept': 'application/json',
				},
				body: JSON.stringify(data),
	  		credentials: "include",
    	})
		.then((response) => {
			if (response.status !== 200) {
				console.log("Status not 200 for workflow publish :O!")
			} else {
				if (isCloud) { 
					alert.success("Successfully published workflow")
				} else {
					alert.success("Successfully published workflow to https://shuffler.io")
				}
			}

			return response.json()
		})
    .then((responseJson) => {
			if (responseJson.reason !== undefined) {
				alert.error("Failed publishing: ", responseJson.reason) 
			}

			getAvailableWorkflows()
    })
		.catch(error => {
			alert.error(error.toString())
		})
	}

	const copyWorkflow = (data) => {
		data = JSON.parse(JSON.stringify(data))
		alert.success("Copying workflow "+data.name)
		data.id = ""
		data.name = data.name+"_copy"
		console.log("COPIED DATA: ", data)
		//return

		fetch(globalUrl+"/api/v1/workflows", {
    	  method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'Accept': 'application/json',
				},
				body: JSON.stringify(data),
	  			credentials: "include",
    		})
		.then((response) => {
			if (response.status !== 200) {
				console.log("Status not 200 for workflows :O!")
				return 
			}
			return response.json()
		})
    .then((responseJson) => {
			getAvailableWorkflows()
    })
		.catch(error => {
			alert.error(error.toString())
		})
	}


	const deleteWorkflow = (id) => {
		fetch(globalUrl+"/api/v1/workflows/"+id, {
    	  method: 'DELETE',
				headers: {
					'Content-Type': 'application/json',
					'Accept': 'application/json',
				},
	  			credentials: "include",
    		})
		.then((response) => {
			if (response.status !== 200) {
				console.log("Status not 200 for setting workflows :O!")
				alert.error("Failed deleting workflow")
			} else {
				alert.success("Deleted workflow "+id)
			}

			return response.json()
		})
		.then((responseJson) => {
			getAvailableWorkflows()
		})
		.catch(error => {
			alert.error(error.toString())
		});
	}


	const handleChipClick = (e) => {
		addFilter(e.target.innerHTML)
	}

	const WorkflowPaper = (props) => {
  	const { data } = props;
		const [open, setOpen] = React.useState(false);
		const [anchorEl, setAnchorEl] = React.useState(null);

		var boxWidth = "2px"
		if (selectedWorkflow.id === data.id) {
			boxWidth = "4px"
		}

		var boxColor = "#FECC00"
		if (data.is_valid) {
			boxColor = "#86c142"
		}

		if (!data.previously_saved) {
			boxColor = "#f85a3e"
		}

		const menuClick = (event) => {
			setOpen(!open)
			setAnchorEl(event.currentTarget);
		}

		var parsedName = data.name
		if (parsedName !== undefined && parsedName !== null && parsedName.length > 25) {
			parsedName = parsedName.slice(0,26)+".." 
		}


		const actions = data.actions !== null ? data.actions.length : 0
		const [triggers, schedules, webhooks, subflows] = getWorkflowMeta(data)

		return (
			<Grid item xs={4} style={{padding: "12px 10px 12px 10px",}}>
				<Paper square style={paperAppStyle}  >	
					<div style={{position: "absolute", bottom: 1, left: 1, height: 12, width: 12, backgroundColor: boxColor, borderRadius: "0 100px 0 0",}} />
					<Grid item style={{display: "flex", flexDirection: "column", width: "100%"}}>
						<Grid item style={{display: "flex", maxHeight: 34,}}>
							<Tooltip title={`Edit ${data.name}`} placement="bottom">
								<Typography variant="body1" style={{marginBottom: 0, paddingBottom: 0, maxHeight: 30, flex: 10,}}>
									<Link to={"/workflows/"+data.id} style={{textDecoration: "none", color: "inherit",}}>
										{parsedName}
									</Link>
								</Typography>
							</Tooltip>
						</Grid>
						<Grid item style={workflowActionStyle}>
							<Tooltip color="primary" title="Action amount" placement="bottom">
								<span style={{color: "#979797", display: "flex"}}>
									<BubbleChartIcon style={{marginTop: "auto", marginBottom: "auto",}} /> 
									<Typography style={{marginLeft: 5, marginTop: "auto", marginBottom: "auto",}}>
										{actions}
									</Typography>
								</span>
							</Tooltip>
							<Tooltip color="primary" title="Trigger amount" placement="bottom">
								<span style={{marginLeft: 15, color: "#979797", display: "flex"}}>
									<RestoreIcon style={{color: "#979797", marginTop: "auto", marginBottom: "auto",}}/> 
									<Typography style={{marginLeft: 5, marginTop: "auto", marginBottom: "auto",}}>
										{triggers}
									</Typography>
								</span>
							</Tooltip>
							<Tooltip color="primary" title="Subflows used" placement="bottom">
								<span style={{marginLeft: 15, display: "flex", color: "#979797", cursor: "pointer"}} onClick={() => {
									if (subflows === 0) {
										alert.info("No subflows for "+data.name)
										return
									}

									var newWorkflows = [data]
									for (var key in data.triggers) {
										const trigger = data.triggers[key]
										if (trigger.app_name !== "Shuffle Workflow") {
											continue
										}

										if (trigger.parameters !== undefined && trigger.parameters !== null && trigger.parameters.length > 0 && trigger.parameters[0].name === "workflow") {
											const newWorkflow = workflows.find(item => item.id === trigger.parameters[0].value)	
											if (newWorkflow !== null && newWorkflow !== undefined) {
												newWorkflows.push(newWorkflow)
												continue
											}
										}
									}

									setFilters(["Subflows of "+data.name])
									setFilteredWorkflows(newWorkflows)
								}}>
									<svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg" style={{color: "#979797", marginTop: "auto", marginBottom: "auto",}}>
										<path d="M0 0H15V15H0V0ZM16 16H18V18H16V16ZM16 13H18V15H16V13ZM16 10H18V12H16V10ZM16 7H18V9H16V7ZM16 4H18V6H16V4ZM13 16H15V18H13V16ZM10 16H12V18H10V16ZM7 16H9V18H7V16ZM4 16H6V18H4V16Z" fill="#979797"/>
									</svg>
									<Typography style={{marginLeft: 5, marginTop: "auto", marginBottom: "auto",}}>
										{subflows}
									</Typography>
								</span>
							</Tooltip>
							{/*
							<Tooltip color="primary" title={`Actions: ${data.actions.length}`} placement="bottom">
								<AppsIcon />
							</Tooltip>
								<Tooltip color="primary" title={`Webhooks: ${webhooks}`} placement="bottom">
										<RestoreIcon />
								</Tooltip>
							: null}
							{schedules > 0 ? 
								<Tooltip color="primary" title={`Schedules: ${schedules}`} placement="bottom">
										<RestoreIcon />
								</Tooltip>
							: null}
							*/}
						</Grid>
						<Grid item style={{justifyContent: "left", overflow: "hidden", marginTop: 5,}}>
							{data.tags !== undefined ?
								data.tags.map((tag, index) => {
									if (index >= 3) {
										return null
									}


									return (
										<Chip
											key={index}
											style={chipStyle}
											label={tag}
											onClick={handleChipClick}
											variant="outlined"
											color="primary"
										/>
									)
								})
							: null}
						</Grid>
					</Grid>
				{data.actions !== undefined && data.actions !== null ? 
					<Grid item style={{display:"flex",flexDirection:"column", justifyContent:"space-between"}}>
						<Grid>
							<IconButton
								aria-label="more"
								aria-controls="long-menu"
								aria-haspopup="true"
								style={{color: "white"}}
								onClick={menuClick}
								style={{padding:"0px",color:"white", color:"#979797"}}
								>
								<MoreVertIcon />
							</IconButton>
							<Menu
								id="long-menu"
								anchorEl={anchorEl}
								keepMounted
								open={open}
								onClose={() => {
									setOpen(false)
									setAnchorEl(null)
								}}
							>
								<MenuItem style={{backgroundColor: inputColor, color: "white"}} onClick={() => {
									setModalOpen(true)
									setEditingWorkflow(data)
									setNewWorkflowName(data.name)
									setNewWorkflowDescription(data.description)
									if (data.tags !== undefined && data.tags !== null) {
										setNewWorkflowTags(JSON.parse(JSON.stringify(data.tags)))
									}
								}} key={"change"}>
									<EditIcon style={{marginLeft: 0, marginRight: 8}}/>
									{"Change details"}
								</MenuItem>
								<MenuItem style={{backgroundColor: inputColor, color: "white"}} onClick={() => {
									publishWorkflow(data) 
								}} key={"publish"}>
									<CloudUploadIcon style={{marginLeft: 0, marginRight: 8}}/>
									{"Publish Workflow"}
								</MenuItem>
								<MenuItem style={{backgroundColor: inputColor, color: "white"}} onClick={() => {
									copyWorkflow(data)		
									setOpen(false)
								}} key={"duplicate"}>
									<FileCopyIcon style={{marginLeft: 0, marginRight: 8}}/>
									{"Duplicate Workflow"}
								</MenuItem>
								<MenuItem style={{backgroundColor: inputColor, color: "white"}} onClick={() => {
									exportWorkflow(data)		
									setOpen(false)
								}} key={"export"}>
									<GetAppIcon style={{marginLeft: 0, marginRight: 8}}/>
									{"Export Workflow"}
								</MenuItem>
								<MenuItem style={{backgroundColor: inputColor, color: "white"}} onClick={() => {
									setDeleteModalOpen(true)
									setSelectedWorkflowId(data.id)
									setOpen(false)
								}} key={"delete"}>
									<DeleteIcon style={{marginLeft: 0, marginRight: 8}}/>
									{"Delete Workflow"}
								</MenuItem>

							</Menu>
						</Grid>
						{/*
						<Grid>
							<Link to={"/workflows/"+data.id}>
								<Tooltip title="Edit workflow" placement="bottom">
									<EditIcon style={{borderRadius: "4px", color: "#F85A3E", height: 20, width: 20, padding: 7, fontSize: "small"}} />
								</Tooltip>
							</Link>	
						</Grid>
						*/}
					</Grid>
					: null}
				</Paper>
			</Grid>
		)
	}



	const dividerColor = "rgb(225, 228, 232)"

	const resultPaperAppStyle = {
		minHeight: "100px",
		minWidth: "100%",
		overflow: "hidden",
		maxWidth: "100%",
		marginTop: "5px",
		color: "white",
		backgroundColor: surfaceColor,
		cursor: "pointer",
		display: "flex",
	}

	function replaceAll(string, search, replace) {
		return string.split(search).join(replace);
	}

	const resultsPaper = (data) => {
		var boxWidth = "2px"
		var boxColor = "orange"
		if (data.status === "ABORTED" || data.status === "UNFINISHED" || data.status === "FAILURE"){
			boxColor = "red"	
		} else if (data.status === "FINISHED" || data.status === "SUCCESS") {
			boxColor = "green"
		} else if (data.status === "SKIPPED" || data.status === "EXECUTING") {
			boxColor = "yellow"
		} else {
			boxColor = "green"
		}

		var t = new Date(data.started_at*1000)
		var showResult = data.result.trim()
		const validate = validateJson(showResult)

		if (validate.valid) {
			showResult = <ReactJson 
				src={validate.result} 
				theme="solarized" 
				collapsed={collapseJson}
				displayDataTypes={false}
				name={"Results for "+data.action.label}
			/>
		} else {
			// FIXME - have everything parsed as json, either just for frontend
			// or in the backend?
			/*	
			const newdata = {"result": data.result}
			showResult = <ReactJson 
				src={JSON.parse(newdata)} 
				theme="solarized" 
				collapsed={collapseJson}
				displayDataTypes={false}
				name={"Results for "+data.action.name}
			/>
			*/
		}

		return (
			<Paper key={data.execution_id} square style={resultPaperAppStyle} onClick={() => {}}>
				<div style={{marginLeft: "10px", marginTop: "5px", marginBottom: "5px", marginRight: "5px", width: boxWidth, backgroundColor: boxColor}}>
				</div>
				<Grid container style={{margin: "10px 10px 10px 10px", flex: "1"}}>
					<Grid style={{display: "flex", flexDirection: "column", width: "100%"}}>
						<Grid item style={{flex: "1"}}>
							<h4 style={{marginBottom: "0px", marginTop: "10px"}}><b>Name</b>: {data.action.label}</h4>
						</Grid>
						<Grid item style={{flex: "1", justifyContent: "center"}}>
							App: {data.action.app_name}, Version: {data.action.app_version}
						</Grid>
						<Grid item style={{flex: "1", justifyContent: "center"}}>
							Action: {data.action.name}, Environment: {data.action.environment}, Status: {data.status}
						</Grid>
						<div style={{display: "flex", flex: "1"}}>
							<Grid item style={{flex: "10", justifyContent: "center"}}>
								Started: {t.toISOString()}
							</Grid>
						</div>
						<Divider style={{marginBottom: "10px", marginTop: "10px", height: "1px", width: "100%", backgroundColor: dividerColor}}/>
						<div style={{display: "flex", flex: "1"}}>
							<Grid item style={{flex: "10", justifyContent: "center"}}>
								{showResult}
							</Grid>
						</div>
					</Grid>
				</Grid>
			</Paper>
		)
	}

	const resultsHandler = Object.getOwnPropertyNames(selectedExecution).length > 0 && selectedExecution.results !== null ? 
		<div>
			{selectedExecution.results.sort((a, b) => a.started_at - b.started_at).map((data, index) => {
				return (
					<div key={index}>
						{resultsPaper(data)}
					</div>
				)
			})}
		</div>
		:
		<div>
			No results yet 
		</div>

	const resultsLength = Object.getOwnPropertyNames(selectedExecution).length > 0 && selectedExecution.results !== null ? selectedExecution.results.length : 0 	

	const ExecutionDetails = () => {
		var starttime = new Date(selectedExecution.started_at*1000)
		var endtime = new Date(selectedExecution.started_at*1000)

		var parsedArgument = selectedExecution.execution_argument
		if (selectedExecution.execution_argument !== undefined && selectedExecution.execution_argument.length > 0) {
			parsedArgument = replaceAll(parsedArgument, " None", " \"None\"");
		}	

		var arg = null
		if (selectedExecution.execution_argument !== undefined && selectedExecution.execution_argument.length > 0) {
			var showResult = selectedExecution.execution_argument.trim()
			const validate = validateJson(showResult)

			arg = validate.valid ? 
				<ReactJson 
					src={validate.result} 
					theme="solarized" 
					collapsed={true}
					displayDataTypes={false}
					name={"Execution argument / webhook"}
				/>
			: showResult
		}

		var lastresult = null
		if (selectedExecution.result !== undefined && selectedExecution.result.length > 0) {
			var showResult = selectedExecution.result.trim()
			const validate = validateJson(showResult)
			lastresult = validate.valid ? 
				<ReactJson 
					src={validate.result} 
					theme="solarized" 
					collapsed={true}
					displayDataTypes={false}
					name={"Last result from execution"}
				/>
			: showResult
		}

		/*
		<div>
			ID: {selectedExecution.execution_id}
		</div>
		<div>
			<b>Last node:</b> {selectedExecution.workflow.actions.find(data => data.id === selectedExecution.last_node).actions[0].label}
		</div>
		*/
		if (Object.getOwnPropertyNames(selectedExecution).length > 0 && selectedExecution.workflow.actions !== null) {
			return (
				<div style={{overflowX: "hidden"}}>
					<div>
						<b>Status:</b> {selectedExecution.status}
					</div>
					<div>
						<b>Started:</b> {starttime.toISOString()}
					</div>
					<div>
						<b>Finished:</b> {endtime.toISOString()}
					</div>
					{/*
					<div>
						<b>Last Result:</b> {lastresult}
					</div>
					*/}
					<div style={{marginTop: 10}}>
						{arg}
					</div>
					<Divider style={{marginBottom: "10px", marginTop: "10px", height: "1px", width: "100%", backgroundColor: dividerColor}}/>
					{resultsHandler}
				</div> 
			)
		}

		return (
			executionLoading ?
				<div style={{marginTop: 25, textAlign: "center"}}>
					<CircularProgress />
				</div>
				: 
				<h4>
					There are no executiondetails yet. Click "execute" to run your first one.
				</h4>
			
		)
	}

	// Can create and set workflows
	const setNewWorkflow = (name, description, tags, editingWorkflow, redirect) => {

		var method = "POST"
		var extraData = ""
		var workflowdata = {}

		if (editingWorkflow.id !== undefined) {
			console.log("Building original workflow")
			method = "PUT"
			extraData = "/"+editingWorkflow.id+"?skip_save=true"
			workflowdata = editingWorkflow

			console.log("REMOVING OWNER")
			workflowdata["owner"] = ""	
			// FIXME: Loop triggers and turn them off?
		}

		workflowdata["name"] = name 
		workflowdata["description"] = description 
		if (tags !== undefined) {
			workflowdata["tags"] = tags 
		}
		//console.log(workflowdata)
		//return

		return fetch(globalUrl+"/api/v1/workflows"+extraData, {
    	  method: method,
				headers: {
					'Content-Type': 'application/json',
					'Accept': 'application/json',
				},
				body: JSON.stringify(workflowdata),
	  			credentials: "include",
    		})
		.then((response) => {
			if (response.status !== 200) {
				console.log("Status not 200 for workflows :O!")
				return 
			}
			return response.json()
		})
    .then((responseJson) => {
			if (method === "POST" && redirect) {
				window.location.pathname = "/workflows/"+responseJson["id"] 
			} else if (!redirect) {
				// Update :)		
				getAvailableWorkflows()
				setImportLoading(false)
			} else { 
				alert.info("Successfully changed basic info for workflow")
			}

			return responseJson
    })
		.catch(error => {
			alert.error(error.toString())
			setImportLoading(false)
		});
	}


	const importFiles = (event) => {
		console.log("Importing!")

		setImportLoading(true)
		const file = event.target.value
		if (event.target.files.length > 0) {
			for (var key in event.target.files) {
				const file = event.target.files[key]
				if (file.type !== "application/json") {
					if (file.type !== undefined) {
						alert.error("File has to contain valid json")
					}

					continue
				}

  			const reader = new FileReader()
				// Waits for the read
	  		reader.addEventListener('load', (event) => {
					var data = reader.result
					try {
						data = JSON.parse(reader.result)
					} catch (e) {
						alert.error("Invalid JSON: "+e)
						setImportLoading(false)
						return
					}

					// Initialize the workflow itself
					const ret = setNewWorkflow(data.name, data.description, data.tags, {}, false)
					.then((response) => {
						if (response !== undefined) {
							// SET THE FULL THING
							data.id = response.id
							data.first_save = false
							data.previously_saved = false 
							data.is_valid = false

							// Actually create it
							const ret = setNewWorkflow(data.name, data.description, data.tags, data, false)
							.then((response) => {
								if (response !== undefined) {
									alert.success("Successfully imported "+data.name)
								}
							})
						}
					})
					.catch(error => {
						alert.error("Import error: "+error.toString())
					});
				})

				// Actually reads
		  	reader.readAsText(file)
			}
		}

		setLoadWorkflowsModalOpen(false)
	}

	const getWorkflowMeta = (data) => {
		let triggers = 0
		let schedules = 0
		let webhooks = 0
		let subflows = 0
		if (data.triggers !== undefined && data.triggers !== null && data.triggers.length > 0) {
			triggers = data.triggers.length
			for (let key in data.triggers) {

				if (data.triggers[key].app_name === "Webhook") {
					webhooks += 1
					//webhookImg = data.triggers[key].large_image
				} else if (data.triggers[key].app_name === "Schedule") {
					schedules += 1
					//scheduleImg = data.triggers[key].large_image
				} else if (data.triggers[key].app_name === "Shuffle Workflow") {
					subflows += 1
				}
			}
		}

		return [triggers, schedules, webhooks, subflows]
	}

	const WorkflowGridView = () => {
			let workflowData = "";
			if (workflows.length > 0) {
				const columns = [
					{ field: 'title', headerName: 'Title', width: 330, },
					{ field: 'actions', headerName: 'Actions', width: 200, sortable: false, 
						disableClickEventBubbling: true,
						renderCell: (params) => {
							const data = params.row.record;
							let [triggers, schedules, webhooks, subflows] = getWorkflowMeta(data);

							return <Grid item>
									<Link to={"/workflows/"+data.id}>
											<EditIcon style={{background: "#F85A3E",
	boxShadow: "0px 1px 2px rgba(0, 0, 0, 0.16), 0px 2px 4px rgba(0, 0, 0, 0.12), 0px 1px 8px rgba(0, 0, 0, 0.1)",
	borderRadius: "4px", color: "black", height: "20px", "width": "20px", fontSize: "small"}} />
									</Link>
									<Tooltip color="primary" title="Edit workflow" placement="bottom">
										<BubbleChartIcon />
									</Tooltip>
									<Tooltip color="primary" title="Execute workflow" placement="bottom">
										<PlayArrowIcon color="secondary" disabled={!data.is_valid} onClick={() => executeWorkflow(data.id)} />
									</Tooltip>
									<Tooltip color="primary" title={`Actions: ${data.actions.length}`} placement="bottom">
										<AppsIcon />
									</Tooltip>
									{webhooks > 0 ? 
										<Tooltip color="primary" title={`Webhooks: ${webhooks}`} placement="bottom">
											<RestoreIcon />
										</Tooltip>
									: null}
									{schedules > 0 ? 
										<Tooltip color="primary" title={`Schedules: ${schedules}`} placement="bottom">
											<RestoreIcon />
										</Tooltip>
									: null}
								</Grid>
							}
					},
					{ field: 'tags', headerName: 'Tags', width: 390, sortable: false, 
						disableClickEventBubbling: true,
						renderCell: (params) => {
							const data = params.row.record;
							return <Grid item>
										{data.tags !== undefined ?
											data.tags.map((tag, index) => {
												if (index >= 3) {
													return null
												}

												return (
													<Chip
														key={index}
														style={chipStyle}
														label={tag}
														variant="outlined"
														color="primary"
													/>
												)
											})
										: null}
									</Grid>
							}
						},
				];
				let rows = [];
				rows = workflows.map((data, index) => {
					let obj = {"id":index+1, "title":data.name, "record":data,};
					return obj;
				});
				workflowData = <DataGrid color="primary" className={classes.root} rows={rows} columns={columns} pageSize={5} checkboxSelection autoHeight density="standard" components={{
					Toolbar: GridToolbar,
				  }} />
			}
			return (
				<div style={gridContainer}>
					{workflowData}
				</div>	
			);
		}

	const modalView = modalOpen ? 
		<Dialog 
			open={modalOpen} 
			onClose={() => {setModalOpen(false)}}
			PaperProps={{
				style: {
					backgroundColor: surfaceColor,
					color: "white",
					minWidth: "800px",
				},
			}}
		>
			<DialogTitle>
				<div style={{color: "rgba(255,255,255,0.9)"}}>
					{editingWorkflow.id !== undefined ? "Editing" : "New"} workflow
					<div style={{float: "right"}}>
						<Tooltip color="primary" title={"Import manually"} placement="top">
							<Button color="primary" style={{}} variant="text" onClick={() => upload.click()}>
								<PublishIcon />
							</Button> 				
						</Tooltip>
					</div>
				</div>
			</DialogTitle>
			<FormControl>
				<DialogContent>
					<TextField
						onBlur={(event) => setNewWorkflowName(event.target.value)}
						InputProps={{
							style:{
								color: "white",
							},
						}}
						color="primary"
						placeholder="Name"
						margin="dense"
						defaultValue={newWorkflowName}
						fullWidth
					  />
					<TextField
						onBlur={(event) => setNewWorkflowDescription(event.target.value)}
						InputProps={{
							style:{
								color: "white",
							},
						}}
						color="primary"
						defaultValue={newWorkflowDescription}
						placeholder="Description"
						margin="dense"
						fullWidth
					  />
					<ChipInput
						style={{marginTop: 10, }}
						InputProps={{
							style:{
								color: "white",
							},
						}}
						placeholder="Tags"
						color="primary"
						fullWidth
						value={newWorkflowTags}
						onAdd={(chip) => {
							newWorkflowTags.push(chip)
							setNewWorkflowTags(newWorkflowTags)
						}}
						onDelete={(chip, index) => {
							newWorkflowTags.splice(index, 1)
							setNewWorkflowTags(newWorkflowTags)
							setUpdate("delete "+chip)
						}}
					/>
				</DialogContent>
				<DialogActions>
					<Button style={{}} onClick={() => {
						setNewWorkflowName("")
						setNewWorkflowDescription("")
						setEditingWorkflow({})
						setNewWorkflowTags([])
						setModalOpen(false)
					}} color="primary">
						Cancel
					</Button>
					<Button style={{}} disabled={newWorkflowName.length === 0} onClick={() => {
						console.log("Tags: ", newWorkflowTags)
						if (editingWorkflow.id !== undefined) {
							setNewWorkflow(newWorkflowName, newWorkflowDescription, newWorkflowTags, editingWorkflow, false)
							setNewWorkflowName("")
							setNewWorkflowDescription("")
							setEditingWorkflow({})
							setNewWorkflowTags([])
						} else {
							setNewWorkflow(newWorkflowName, newWorkflowDescription, newWorkflowTags, {}, true)
						}

						setModalOpen(false)
					}} color="primary">
	        	Submit	
	        </Button>
				</DialogActions>
			</FormControl>
		</Dialog>
		: null

		
		const viewSize = {
			workflowView: 4,
			executionsView: 3,
			executionResults: 4,
		}

		const workflowViewStyle = {
			flex: viewSize.workflowView, 
			marginLeft: "10px", 
			marginRight: "10px", 
		}

		if (viewSize.workflowView === 0) {
			workflowViewStyle.display = "none"
		}

		const workflowButtons = 
			<span>
				{workflows.length > 0 ?
					<Tooltip color="primary" title={"Create new workflow"} placement="top">
						<Button color="primary" style={{}} variant="text" onClick={() => setModalOpen(true)}><AddIcon /></Button> 				
					</Tooltip>
				: null}
				<Tooltip color="primary" title={"Import workflows"} placement="top">
					{importLoading ? 
						<Button color="primary" style={{}} variant="text" onClick={() => {}}>
							<CircularProgress style={{maxHeight: 15, maxWidth: 15,}} />
						</Button> 				
						: 
						<Button color="primary" style={{}} variant="text" onClick={() => upload.click()}>
							<PublishIcon />
						</Button> 				
					}
				</Tooltip>
				<input hidden type="file" multiple="multiple" ref={(ref) => upload = ref} onChange={importFiles} />
				{workflows.length > 0 ? 
					<Tooltip color="primary" title={`Download ALL workflows (${workflows.length})`} placement="top">
						<Button color="primary" style={{}} variant="text" onClick={() => {
							exportAllWorkflows()
						}}>
							<GetAppIcon />
						</Button> 				
					</Tooltip>
				: null}
				{isCloud ? null :
				<Tooltip color="primary" title={"Download workflows"} placement="top">
					<Button color="primary" style={{}} variant="text" onClick={() => setLoadWorkflowsModalOpen(true)}>
						<CloudDownloadIcon />
					</Button> 				
				</Tooltip>
				}
			</span>

		const WorkflowView = () => {
			if (workflows.length === 0) {
				return (
					<div style={emptyWorkflowStyle}>	
						<Paper style={boxStyle}>
							<div>
								<h2>Welcome to Shuffle</h2>
							</div>
							<div>
								<p>
									<b>Shuffle</b> is a flexible, easy to use, automation platform allowing users to integrate their services and devices freely. It's made to significantly reduce the amount of manual labor, and is focused on security applications. <a href="/docs/about" style={{textDecoration: "none", color: "#f85a3e"}}>Click here to learn more.</a>
								</p>
							</div>
							<div>
								If you want to jump straight into it, click here to create your first workflow: 
							</div>
							<div style={{display: "flex"}}>
								<Button color="primary" style={{marginTop: "20px",}} variant="outlined" onClick={() => setModalOpen(true)}>New workflow</Button> 				
								<span style={{paddingTop: 20, display: "flex",}}>
									<Typography style={{marginTop: 5, marginLeft: 30, marginRight: 15}}>
										..OR
									</Typography>
									{workflowButtons}
								</span>
							</div>
						</Paper>
					</div>
				)
			}

		return (
			<div style={viewStyle}>	
				<div style={workflowViewStyle}>
					<div style={{display: "flex"}}>
						<div style={{flex: 3}}>
							<h2>Workflows</h2>
						</div>
					</div>

					{/*
					<div style={flexContainerStyle}>
						<div style={{...flexBoxStyle, ...activeWorkflowStyle}}>
							<div style={flexContentStyle}>
								<div ><img src={mobileImage} style={iconStyle} /></div>
								<div style={ blockRightStyle }>
									<div style={counterStyle}>{workflows.length}</div>
									<div style={fontSize_16}>ACTIVE WORKFLOWS</div>
								</div>
							</div>
						</div>
						<div style={{...flexBoxStyle, ...availableWorkflowStyle}}>
							<div style={flexContentStyle}>
								<div><img src={bookImage} style={iconStyle} /></div>
								<div style={ blockRightStyle }>
									<div style={counterStyle}>{workflows.length}</div>
									<div style={fontSize_16}>AVAILABE WORKFLOWS</div>
								</div>
							</div>
						</div>
						<div style={{...flexBoxStyle, ...notificationStyle}}>
							<div style={flexContentStyle}>
								<div><img src={bagImage} style={iconStyle} /></div>
								<div style={ blockRightStyle }>
									<div style={counterStyle}>{workflows.length}</div>
									<div style={fontSize_16}>NOTIFICATIONS</div>
								</div>
							</div>
						</div>
					</div>
					*/}

					{/*
					chipRenderer={({ value, isFocused, isDisabled, handleClick, handleRequestDelete }, key) => {
						console.log("VALUE: ", value)

						return (
							<Chip
								key={key}
								style={chipStyle}

							>
								{value}
							</Chip>
						)
					}}
					*/}
					<div style={{display: "flex", margin: "0px 0px 20px 0px"}}>
						<div style={{flex: 1}}>
							<Typography style={{marginTop: 7, marginBottom: "auto"}}>
								<a rel="norefferer" target="_blank" href="https://shuffler.io/docs/workflows" target="_blank" style={{textDecoration: "none", color: "#f85a3e"}}>Learn more about Workflows</a>
							</Typography>
						</div>
						<div style={{flex: 1, float: "right",}}>
							<ChipInput
								style={{}}
								InputProps={{
									style:{
										color: "white",
										//backgroundColor: inputColor,
									},
								}}
								placeholder="Filter Your Workflows"
								color="primary"
								fullWidth
								value={filters}
								onAdd={(chip) => {
									addFilter(chip)
								}}
								onDelete={(chip, index) => {
									removeFilter(index)
								}}
							/>
						</div>
						<div style={{float: "right", flex: 1, textAlign: "right",}}>
							{workflowButtons}
						</div>
					</div>
					<div style={{marginTop: 15,}} />
					{view === "grid" && (
						<Grid container spacing={4} style={paperAppContainer}>
							{filteredWorkflows.map((data, index) => {
								return (
									<WorkflowPaper key={index} data={data} />
								)
							})}
						</Grid>
					)}
					
					{view === "list" && (
						<WorkflowGridView />
					)}

					<div style={{marginBottom: 100}}/>
				</div>
			</div>
		)
	}

	const importWorkflowsFromUrl = (url) => {
		console.log("IMPORT WORKFLOWS FROM ", downloadUrl)

		const parsedData = {
			"url": url,
			"field_3": downloadBranch || 'master'
		}

		if (field1.length > 0) {
			parsedData["field_1"] = field1
		}

		if (field2.length > 0) {
			parsedData["field_2"] = field2
		}

		alert.success("Getting specific workflows from your URL.")
		var cors = "cors"
		fetch(globalUrl+"/api/v1/workflows/download_remote", {
    	method: "POST",
			mode: "cors",
			headers: {
				'Accept': 'application/json',
			},
			body: JSON.stringify(parsedData),
	  	credentials: "include",
		})
		.then((response) => {
			if (response.status === 200) {
				alert.success("Successfully loaded workflows from "+downloadUrl)
				getAvailableWorkflows()
			}

			return response.json()
		})
    .then((responseJson) => {
				console.log("DATA: ", responseJson)
				if (!responseJson.success) {
					if (responseJson.reason !== undefined) {
						alert.error("Failed loading: "+responseJson.reason)
					} else {
						alert.error("Failed loading")
					}
				}
		})
		.catch(error => {
			alert.error(error.toString())
		})
	}

	const handleGithubValidation = () => {
		importWorkflowsFromUrl(downloadUrl)
		setLoadWorkflowsModalOpen(false)
	}

	const workflowDownloadModalOpen = loadWorkflowsModalOpen ? 
		<Dialog 
			open={loadWorkflowsModalOpen}
			onClose={() => {
			}}
			PaperProps={{
				style: {
					backgroundColor: surfaceColor,
					color: "white",
					minWidth: "800px",
					minHeight: "320px",
				},
			}}
		>
			<DialogTitle>
				<div style={{color: "rgba(255,255,255,0.9)"}}>
					Load workflows from github repo
					<div style={{float: "right"}}>
						<Tooltip color="primary" title={"Import manually"} placement="top">
							<Button color="primary" style={{}} variant="text" onClick={() => upload.click()}>
								<PublishIcon />
							</Button> 				
						</Tooltip>
					</div>
				</div>
			</DialogTitle>
			<DialogContent style={{color: "rgba(255,255,255,0.65)"}}>
				Repository (supported: github, gitlab, bitbucket)
				<TextField
					style={{backgroundColor: inputColor}}
					variant="outlined"
					margin="normal"
					defaultValue={userdata.active_org.defaults.workflow_download_repo !== undefined && userdata.active_org.defaults.workflow_download_repo.length > 0 ? userdata.active_org.defaults.workflow_download_repo : downloadUrl}
					InputProps={{
						style:{
							color: "white",
							height: "50px",
							fontSize: "1em",
						},
					}}
					onChange={e => setDownloadUrl(e.target.value)}
					placeholder="https://github.com/frikky/shuffle-apps"
					fullWidth
				/>

					<span style={{marginTop: 10}}>Branch (default value is "master"):</span>
					<div style={{display: "flex"}}>
						<TextField
							style={{backgroundColor: inputColor}}
							variant="outlined"
							margin="normal"
							defaultValue={userdata.active_org.defaults.workflow_download_branch !== undefined && userdata.active_org.defaults.workflow_download_branch.length > 0 ? userdata.active_org.defaults.workflow_download_branch : downloadBranch}
							InputProps={{
								style:{
									color: "white",
									height: "50px",
									fontSize: "1em",
								},
							}}
							onChange={e => setDownloadBranch(e.target.value)}
							placeholder="master"
							fullWidth
							/>
					</div>

				<span style={{marginTop: 10}}>Authentication (optional - private repos etc):</span>
				<div style={{display: "flex"}}>
					<TextField
						style={{flex: 1, backgroundColor: inputColor}}
						variant="outlined"
						margin="normal"
						InputProps={{
							style:{
								color: "white",
								height: "50px",
								fontSize: "1em",
							},
						}}
						onChange={e => setField1(e.target.value)}
						type="username"
						placeholder="Username / APIkey (optional)"
						fullWidth
						/>
					<TextField
						style={{flex: 1, backgroundColor: inputColor}}
						variant="outlined"
						margin="normal"
						InputProps={{
							style:{
								color: "white",
								height: "50px",
								fontSize: "1em",
							},
						}}
						onChange={e => setField2(e.target.value)}
						type="password"
						placeholder="Password (optional)"
						fullWidth
						/>
				</div>
			</DialogContent>
			<DialogActions>
				<Button style={{borderRadius: "0px"}} onClick={() => setLoadWorkflowsModalOpen(false)} color="primary">
					Cancel
				</Button>
	      <Button style={{borderRadius: "0px"}} disabled={downloadUrl.length === 0 || !downloadUrl.includes("http")} onClick={() => {
					handleGithubValidation() 
				}} color="primary">
	        Submit	
	      </Button>
			</DialogActions>
		</Dialog>
		: null

	const loadedCheck = isLoaded && isLoggedIn && workflowDone ? 
		<div>
			<Dropzone style={{maxWidth: window.innerWidth > 1366 ? 1366 : 1200, margin: "auto", padding: 20 }} onDrop={uploadFile}>
				<WorkflowView />
			</Dropzone>
			{modalView}
			{deleteModal}
			{workflowDownloadModalOpen}
		</div>
		:
		<div style={{paddingTop: 250, width: 250, margin: "auto", textAlign: "center"}}>
			<CircularProgress />
			<Typography>
				Loading Workflows
			</Typography>
		</div>


	// Maybe use gridview or something, idk
	return (
		<div>
			{loadedCheck}
		</div>
	)
}

export default Workflows 

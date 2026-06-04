package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/ps78674/docopt.go"
)

const configFile = "config.json"

var (
	indent       int
	vendors      []string          // requested vendors (comma-separated for discovery)
	toolBinaries map[string]string // vendor -> raid tool binary
	operation    string
	argOption    string
	controllerID string
	deviceID     string
)

func init() {
	type Config struct {
		Vendors interface{} `json:"vendors"`
	}

	var (
		configJSON       Config
		availableVendors []string
		discoveryOption  string
		statusOption     string
		options          []string
	)

	ex, err := os.Executable()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	configFile, err := os.Open(fmt.Sprintf("%s/%s", filepath.Dir(ex), configFile))
	if err != nil {
		fmt.Printf("Error opening config file: %s\n", err)
		os.Exit(1)
	}

	configData, err := ioutil.ReadAll(configFile)
	if err != nil {
		fmt.Printf("Error reading config file: %s\n", err)
		os.Exit(1)
	}

	if err := json.Unmarshal(configData, &configJSON); err != nil {
		fmt.Printf("Error unmarshalling JSON data: %s\n", err)
		os.Exit(1)
	}

	if configJSON.Vendors == nil {
		fmt.Println("Failed to get vendors from config file.")
		os.Exit(1)
	}

	binaryMap := configJSON.Vendors.(map[string]interface{})
	for v := range binaryMap {
		availableVendors = append(availableVendors, v)
	}

	discoveryOptions := []string{"ct", "ld", "pd"}
	statusOptions := []string{"ct,<CONTROLLER_ID>", "ld,<CONTROLLER_ID>,<LD_ID>", "pd,<CONTROLLER_ID>,<PD_ID>"}

	var programName = filepath.Base(os.Args[0])
	var usage = fmt.Sprintf(`%[1]s: parse raid vendor tool output and format it as json

Usage:
  %[1]s (-v <VENDOR>) (-d <OPTION> | -s <OPTION>) [-i <INT>]

Options:
  -v, --vendor <VENDOR>    raid tool vendor, one of: %[2]s
                           (comma-separated for discovery, e.g. megacli,mdstat)
  -d, --discover <OPTION>  discovery option, one of: %[3]s
  -s, --status <OPTION>    status option, one of: %[4]s
  -i, --indent <INT>       indent json output level [default: 0]

  -h, --help               show this screen
	`, programName, strings.Join(availableVendors, " | "), strings.Join(discoveryOptions, " | "), strings.Join(statusOptions, " | "))

	cmdOpts, err := docopt.ParseDoc(usage)
	if err != nil {
		fmt.Printf("error parsing options: %s\n", err)
		os.Exit(1)
	}

	vendorArg, _ := cmdOpts.String("--vendor")
	discoveryOption, _ = cmdOpts.String("--discover")
	statusOption, _ = cmdOpts.String("--status")
	indent, _ = cmdOpts.Int("--indent")

	toolBinaries = map[string]string{}
	for _, v := range strings.Split(vendorArg, ",") {
		v = strings.TrimSpace(v)
		if len(v) == 0 {
			continue
		}

		binary, ok := binaryMap[v]
		if !ok {
			fmt.Printf("Vendors must be one of '%s' (ex.: -v adaptec), got '%s'.\n", strings.Join(availableVendors, " | "), v)
			docopt.PrintHelpOnly(nil, usage)
			os.Exit(1)
		}

		vendors = append(vendors, v)
		toolBinaries[v] = binary.(string)
	}

	if len(vendors) == 0 {
		fmt.Println("No vendor specified.")
		docopt.PrintHelpOnly(nil, usage)
		os.Exit(1)
	}

	if len(discoveryOption) != 0 {
		operation = "Discovery"
		options = discoveryOptions
		argOption = discoveryOption
	} else if len(statusOption) != 0 {
		operation = "Status"
		options = statusOptions
		argOption = statusOption
	}

	for i, v := range options {
		rangeValues := strings.Split(v, ",")
		argOptionValues := strings.SplitN(argOption, ",", 3)

		if argOptionValues[0] != rangeValues[0] || len(argOptionValues) != len(rangeValues) {
			if i == len(options)-1 {
				fmt.Printf("%s option must be one of '%s', got '%s'.\n", operation, strings.Join(options, " | "), argOption)
				docopt.PrintHelpOnly(nil, usage)
				os.Exit(1)
			}

			continue
		}

		if len(argOptionValues) > 1 {
			argOption = argOptionValues[0]
		}

		if len(argOptionValues) == 2 || len(argOptionValues) == 3 {
			controllerID = argOptionValues[1]
		}

		if len(argOptionValues) == 3 {
			controllerID = argOptionValues[1]
			deviceID = argOptionValues[2]
		}

		break
	}

	// Status targets a single discovered element, so it must name exactly one vendor.
	if operation == "Status" && len(vendors) != 1 {
		fmt.Printf("Status requires a single vendor, got '%s'.\n", strings.Join(vendors, ","))
		os.Exit(1)
	}
}

// openPlugin - load the shared object for a vendor from the executable's directory.
func openPlugin(vendor string) *plugin.Plugin {
	ex, err := os.Executable()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	p, err := plugin.Open(filepath.Dir(ex) + "/" + vendor + ".so")
	if err != nil {
		fmt.Printf("Error opening plugin '%s.so': %s\n", vendor, err)
		os.Exit(1)
	}

	return p
}

// lookup - resolve an exported plugin symbol or exit.
func lookup(p *plugin.Plugin, name string) plugin.Symbol {
	sym, err := p.Lookup(name)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return sym
}

// writeJSON - marshal v (honouring the indent flag) and write it to stdout.
func writeJSON(v interface{}) {
	var (
		JSON []byte
		jErr error
	)

	if indent > 0 {
		JSON, jErr = json.MarshalIndent(v, "", strings.Repeat(" ", indent))
	} else {
		JSON, jErr = json.Marshal(v)
	}

	if jErr != nil {
		fmt.Println(jErr)
		os.Exit(1)
	}

	os.Stdout.Write(append(JSON, "\n"...))
}

func discoverControllers() {
	type Element struct {
		Vendor string `json:"{#VENDOR}"`
		CT     string `json:"{#CT_ID}"`
	}
	type Reply struct {
		Data []Element `json:"data"`
	}

	d := []Element{}
	for _, vendor := range vendors {
		p := openPlugin(vendor)
		getControllersIDs := lookup(p, "GetControllersIDs").(func(string) []string)

		for _, ctID := range getControllersIDs(toolBinaries[vendor]) {
			d = append(d, Element{Vendor: vendor, CT: ctID})
		}
	}

	writeJSON(Reply{d})
}

func discoverLogicalDrives() {
	type Element struct {
		Vendor string `json:"{#VENDOR}"`
		CT     string `json:"{#CT_ID}"`
		LD     string `json:"{#LD_ID}"`
	}
	type Reply struct {
		Data []Element `json:"data"`
	}

	d := []Element{}
	for _, vendor := range vendors {
		p := openPlugin(vendor)
		getControllersIDs := lookup(p, "GetControllersIDs").(func(string) []string)
		getLogicalDrivesIDs := lookup(p, "GetLogicalDrivesIDs").(func(string, string) []string)

		for _, ctID := range getControllersIDs(toolBinaries[vendor]) {
			for _, ldID := range getLogicalDrivesIDs(toolBinaries[vendor], ctID) {
				d = append(d, Element{Vendor: vendor, CT: ctID, LD: ldID})
			}
		}
	}

	writeJSON(Reply{d})
}

func discoverPhysicalDrives() {
	type Element struct {
		Vendor string `json:"{#VENDOR}"`
		CT     string `json:"{#CT_ID}"`
		PD     string `json:"{#PD_ID}"`
	}
	type Reply struct {
		Data []Element `json:"data"`
	}

	d := []Element{}
	for _, vendor := range vendors {
		p := openPlugin(vendor)
		getControllersIDs := lookup(p, "GetControllersIDs").(func(string) []string)
		getPhysicalDrivesIDs := lookup(p, "GetPhysicalDrivesIDs").(func(string, string) []string)

		for _, ctID := range getControllersIDs(toolBinaries[vendor]) {
			for _, pdID := range getPhysicalDrivesIDs(toolBinaries[vendor], ctID) {
				d = append(d, Element{Vendor: vendor, CT: ctID, PD: pdID})
			}
		}
	}

	writeJSON(Reply{d})
}

func getControllerStatus(controllerID string) {
	vendor := vendors[0]
	p := openPlugin(vendor)
	getControllerStatus := lookup(p, "GetControllerStatus").(func(string, string, int) []byte)
	os.Stdout.Write(getControllerStatus(toolBinaries[vendor], controllerID, indent))
}

func getLDStatus(controllerID string, deviceID string) {
	vendor := vendors[0]
	p := openPlugin(vendor)
	getLDStatus := lookup(p, "GetLDStatus").(func(string, string, string, int) []byte)
	os.Stdout.Write(getLDStatus(toolBinaries[vendor], controllerID, deviceID, indent))
}

func getPDStatus(controllerID string, deviceID string) {
	vendor := vendors[0]
	p := openPlugin(vendor)
	getPDStatus := lookup(p, "GetPDStatus").(func(string, string, string, int) []byte)
	os.Stdout.Write(getPDStatus(toolBinaries[vendor], controllerID, deviceID, indent))
}

func main() {
	switch argOption {
	case "ct":
		switch operation {
		case "Discovery":
			discoverControllers()
		case "Status":
			getControllerStatus(controllerID)
		}
	case "ld":
		switch operation {
		case "Discovery":
			discoverLogicalDrives()
		case "Status":
			getLDStatus(controllerID, deviceID)
		}
	case "pd":
		switch operation {
		case "Discovery":
			discoverPhysicalDrives()
		case "Status":
			getPDStatus(controllerID, deviceID)
		}
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"io/ioutil"
	"time"
	"flag"
)


// Constant pattern used for formatting time
const dateFormatterPattern string = "2006/1/2"

// Set of defaults used if no command line overrides are provided
var defaultRetentionConfigFileName string = "config.json"
var defaultTopLevelDataDirName string = "/data"
var verboseLogEnabled bool

// This struct is used to unmarshal Retention data from config.json file
type RetentionConfig struct {
	Retention map[string]int `json:"retention"`
}

// This stores parsed out config.json in memory for O(1) lookup. Here key is companyID and value is retention in days.
var companyDataRetentionMap map[string]int

// Slice to store list of directories which is past retention and need to be deleted.
var dirsToBeDeleted []string

// These variables are used to store temporary meta data while recursively walking down the filesystem directory structure
var companyId, year, month, date string
var currentDate time.Time

// This program is used to delete all the minute level files which has passed the configured retention time in days for each company.
// It first parses the config data for retention information for each company and then walks down the provided data files directory structure
// recursively and finds the directores which has past retention time. Last step, it delete the list of directories and it's content files
// which has past retention period.
func main() {

	// Few command like flags to control the execution of program without making code changes, for eg for things like debugging, etc.
	retentionConfigFileName := flag.String("retentionConfigFileName", defaultRetentionConfigFileName, 
		"Name of config file holding Company retention data in days, eg, config.json")
	topLevelDataDirName := flag.String("topLevelDataDirName", defaultTopLevelDataDirName, 
		"Top level directory where monitoring data is located, eg, /data")
	verboseLogEnabledPtr := flag.Bool("enableVerboseLogging", false, "Enable verbose logging for debugging")
	
	flag.Parse()
	verboseLogEnabled = *verboseLogEnabledPtr
	log.Printf("Command Line Flags Dump => retentionConfigFileName: %s ; topLevelDataDirName: %s ; verboseLogEnabled: %t", *retentionConfigFileName, 
		*topLevelDataDirName, verboseLogEnabled)

	// this is the current time used to calculate if files are expired. This is calculated when this program starts and used throughout the
	// program.
	currentDate = time.Now()

	loadRetentionConfig(*retentionConfigFileName)
	log.Printf("Dump of Retention config: %v\n\n", companyDataRetentionMap)

	determineFilesToBeDeleted(*topLevelDataDirName)
	log.Println(" ****************************************** \n")
	log.Printf("Number of expired directories up for deletion: %d\n", len(dirsToBeDeleted))
	for _, dir := range dirsToBeDeleted {
		dirDeleteErr := os.RemoveAll(dir)
		if  dirDeleteErr != nil {
			// For production use cases this is the case where we should have some metric monitoring and alerting for cases where files are 
			// not being deleted leading to storage space leak
			log.Printf("Failed to delete this dir: [%s] with error %v", dir, dirDeleteErr)
			continue
		}

		log.Println("DELETED EXPIRED DIR: ", dir)
	}
}

// This function reads provide Config file path from disc and stores the parsed out data as a HashMap (companyDataRetentionMap) in memory.
// This companyDataRetentionMap has companyID as key and retention in days as value.
func loadRetentionConfig(configFilePath string) {

	file, err := os.Open(configFilePath)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}

	defer file.Close()

	fileContent, fileReadErr := ioutil.ReadAll(file)
	if fileReadErr != nil {
		log.Fatalf("Failed to read config file: %v", fileReadErr)
	}

	var retentionConfig RetentionConfig
	err = json.Unmarshal(fileContent, &retentionConfig)
    if err != nil {
        log.Fatalf("Failed to unmarshal retention config: %v", err)
    }

    companyDataRetentionMap = retentionConfig.Retention
}

// This function is responsible for walking down the given directory structure and check the directory path to find out if any
// directorty has expired. It stores expired directory names in dirsToBeDeleted slice for later deletion.
func determineFilesToBeDeleted(topDir string) {
	determineFilesToBeDeletedHelper(topDir, 1)
}

// Helper function which recursively walks down the directory structure. Here nestingLevel is used to derive meta data from directory path name.
// For eg, nestingLevel=1 is companyID, nestingLevel=2 deviceID, etc.
func determineFilesToBeDeletedHelper(topDir string, nestingLevel int) {

	// Based on my design, I am only going 5 level deep in directory nesting. This is an optimization as I already know the age of files as soon as 
	// I am able to see the dates in yyyy/mm/dd level. I don't need to check hour and minute level data. As all hour and minute data will anyway 
	// be expired if toplevel day dir is past retention. In this way, I can easily delete the entire directory instead of going file by file which will
	// have huge cost in terms of Disc I/O.
	if nestingLevel > 5 {
		
		retention, ok := companyDataRetentionMap[companyId]
		if !ok {
			retention = companyDataRetentionMap["default"]
		}

		if verboseLogEnabled {
			log.Printf("***********CompanyID: %s ; Date: %s/%s/%s ; Retention: %d **************\n\n", companyId, year, month, date, retention)	
		}

		dirTimeStampDateAsStr := fmt.Sprintf("%s/%s/%s", year, month, date)
		dirTimeStampDat, err := time.Parse(dateFormatterPattern, dirTimeStampDateAsStr)
		if err != nil {
			log.Fatalf("Error parsing directory date: %v", err)
		}
		diffDays := dateDiff(currentDate, dirTimeStampDat)

		if (diffDays > retention) {
			dirsToBeDeleted = append(dirsToBeDeleted, topDir)
			if verboseLogEnabled {
				log.Println("EXPIRED DIRS FOUND -> dirList len: ", len(dirsToBeDeleted), " ;dirsToBeDeleted: " , dirsToBeDeleted)
			}
		}

		return
	}

	dirs, err := os.ReadDir(topDir)
    if err != nil {
        if verboseLogEnabled {
        	log.Printf("Informational Log -> Failed to read dir: %v", err)
    	}
        return
    }

    for _, dirEntry := range dirs {
    	dirName := dirEntry.Name()

    	// ignore hidden files
    	if (dirName[0] == '.') {
    		continue
    	}

    	// TODO: Could replace this with Switch statement to prevent too many if-else blocks
    	if nestingLevel == 1 {
    		companyId = dirName
    		if verboseLogEnabled {
    			log.Printf("\n\n\n===========Company Name: %s ===========================", dirName)
    		}
    	} else if nestingLevel == 3 {
    		year = dirName
    	} else if nestingLevel == 4 {
    		month = dirName
    	} else if nestingLevel == 5 {
    		date = dirName
    	}

    	nextFilePath := fmt.Sprintf("%v/%v", topDir, dirName)
		if verboseLogEnabled {
			log.Println(nextFilePath)
		}
		
		determineFilesToBeDeletedHelper(nextFilePath, nestingLevel + 1)
    }
}

// This function returns difference between 2 dates in terms of number of days
func dateDiff(currentDate time.Time, otherDate time.Time) int {
	
	// calculates the difference of 2 given dates in days
	diffDays := int(currentDate.Sub(otherDate).Hours() / 24)

	return diffDays
}
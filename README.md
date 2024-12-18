# fs-cleaner
This repository hosts a tool which deletes older files which are past given retention time period in days.

**Dependencies:**
- This tool relies on config.json to parse out Retention data in days for each company. By default, it will use provided config.json in current directory or you can use a command line flag ```retentionConfigFileName``` to pass location of a different config file.
- This tool looks for files to be deleted under /data directory. This is the default location but if your files are located elsewhere on disc then you can provide custom location of data directory with command line flag ```topLevelDataDirName```. Custom value of topLevelDataDirName must be upto ```data``` dirname, for eg, /Users/ricky/data in my case.

**How to build and run this program:**
- Clone this repo using this command: ```git clone git@github.com:prabhash1785/fs-gc.git```
- Make sure you have Golang compiler installed on your machine. Check Golang compiler version using this command: ```go version```
  -   Note: I built and ran this program using Go version - ```go version go1.22.3 darwin/arm64```
-   cd to this directory - ```fs-gc```
-   Compile using this command: ```go build fs_cleaner.go```
-   Run this program as follows: ```./fs_cleaner```
  - _Note:To check the command line flags supported to provide custom location of /data directory and config.json, run ```./fs_cleaner --help```_


**Design Choices:**
- Instead of recursively getting to each data file and index files under minute level directories and deleting them one at a time, I have chosen to only go 5 level deep in data directory structure, 5 levels being ```comanyID (level 1)/deviceID (level 2)/year (level 3)/month (level 4)/day (level 5)```. The reason for making this design choice is as follows:
  - Since our retention time is given in days not in hours or minutes, we can easily assert based on directory path that all the hour and minute level directories and files under a given date/day is either expired or non-expired depending upon configured retention time for the company. As a result, instead of iterating over millions of files for each day, we just delete top level directories which are in mere hundreds for each company per day. Letting Operating System delete top level directory is far more efficient in terms of Disc I/O then deleting millions of files one at a time (millions of Disc Seeks).
  - Below math shows the order of magnitude difference in deletes based on my design vs deleting one file at a time.
    - Number of files per company per day = 200 (devices) * 50 (files under per minute dir) * 60 (per hour) * 24 (per day) = 14.4 M  -> this is how many deletes we will do if we go to each file for each company. This would be multiplied by nuber of companies' files on host.
    - On the other hand with my design choice, we just need to delete these many directories per day per company = 200 * 1 (Number of devices per company). Again for n companies on host, this will be 200 * n deletes (max) per day. This is 72000 times more efficient (72000 less Disc Seeks) than going to each file.<br>
- When this program will run: I propose to run this program once in a day as a cron preferably during low traffic hours like mid-night (it really depends on where machines are located and what their low traffic hours are). I chose daily to keep disc space free by cleaning up files everyday and also not have to do 7x cleanup if we have to run once in a week adding 7x the load on disc.
- How to not interfere with other processes: I suggest to run this program as a low priority Linux process so that OS executes this cron as low priority process compared to other processes on machine. I haven't made this code change in this tool but this can be easily set in code to run as low priority process.
- Also while deleting files and directories, introduce a delay (like a sleep of a few configured seconds) to not cause a huge spike in Disc I/O.

**Things I would do if I spend more time:**
- Make this tool more efficent and less bursty by doing following:
  - Introduce a configured amount of sleep in seconds between each directory delete to reduce pressure on Disc I/O.
  - Configure this process to run as a low priority Linux process.
- Delete directories with no files in it. For now, my code leaves directories like month intact even though it doesn't have any files left. I would like to fix it if I spend some more time.
- Write unit tests.
- Run linter on code to find and fix linting errors.
- Use less globals than I have done in this standalone program. Globals could lead to tricky bugs and hard to test code, plus it can introdude side effects. I would fix these before I run this code in prod.
- Fix path in my Go module for it to be downloadable.
- Use logs with fine grained levels for more controlled logging.

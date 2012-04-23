# About #

A program that monitors a directory for file names matching keywords specified in a configuration file and copies files to another location.

# Installation #

1. If monitoring a remote host over SSHFS

## Linux

1. Install [sshfs](http://fuse.sourceforge.net/sshfs.html)
2. Mount remote host.

## Windows


1. Install [Dokan library](http://dokan-dev.net/en/download/#dokan) and [Dokan SSHFS](http://dokan-dev.net/en/download/#sshfs)
1. Mount remote host.

## OSX

If monitoring a remote host over SSHFS:

1. Install [MacFusion](http://macfusionapp.org/)
1. Mount remote host.


# Configuration #

The configuration file `syncgenie.ini` must reside in the same location as the binary.

There is a main `[SyncGenie]` section which must contain at least

    watch_directory = /some/location

You may have as many other sections as you'd like.

    [Whatever]
    keywords = this, (that|other) / another, group, of, matches
    destination = /overthere
    
When a file contains the keywords, `this`, `that`, and `other` (case insensitive search) the file will be copied in 32 KB chunks to `destination`.

## Required

### [SyncGenie]

* watch_directory (string) - Operating System path, if on Windows double up backslashes, e.g. C:\\

### Sections

* keywords (string) - List of words separated by commads. Will match on all keywords. You a / to divide up sections. Regular expressions supported
* destination (string) - Operating System path, if on Windows double up backslashes, e.g. C:\\

## Optional

### [SyncGenie]

* watch\_directory\_poll (integer) - Number of seconds to poll directory
* watch\_directory\_max\_depth (interger) - -1 means full depth, otherwise limit directory descent by n
* run\_when\_done (string) - Executable with arguments to run when a download completes
* concurrent_copies (integer) - Number of concurrent copies that may happen at one time
* verbose_listing (boolean) - Whether or not to list files during index time
* age\_before\_copy (integer) - Number of seconds to wait for a file to stop being modified

### Sections

* keywords_directories (string) - List of words separated by commads. Will match directory names. Regular expressions supported

# Running #

## Linux / OSX

    $ ./syncgenie
    
## Windows

1. Open a command prompt
1. Navigate to binary location
1. Run `syncgenie.exe`

# Author #

Adam Flott, adam@npjh.com

package main

/* TODO

 * re-do ini loader to not rely on a thirdparty lib
 * http / rpc interface

 */

import (
        "encoding/json"
        "flag"
        "fmt"
        "io"
        "io/ioutil"
        "log"
        "math"
        "os"
        "os/exec"
        "os/signal"
        "path"
        "path/filepath"
        "regexp"
        "runtime"
        "sort"
        "strings"
        "sync"
        "time"
)

import (
        "github.com/howeyc/fsnotify"
        "github.com/glacjay/goini"
)

type SyncGenieKeywords struct {
        keywords []string
        keywords_directories []string
        destination string
}

type SyncGenieCopyItem struct {
        from string
        to string
        remaining int64
}

type SyncGenieConfig struct {
        watch_directory *string
        watch_directory_max_depth int
        watch_directory_poll time.Duration
        run_when_done string
        keywords map[string]SyncGenieKeywords
        concurrent_copies int
        verbose_listing bool
        age_before_copy time.Duration
}

type SyncGenieCopyProgress struct {
        file string
        progress int
}

var sync_genie_config_file *string
var sync_genie_config SyncGenieConfig
var sync_genie_config_loaded bool
var sync_genie_config_loading_lock sync.Mutex

var sync_genie_chan_new_files chan SyncGenieCopyItem
var sync_genie_currently_copying map[string]int64
var sync_genie_copy_buffer_size int64 = 32 * 1024

var sync_genie_history_file string
var sync_genie_history []string

func SyncGenieLoadConfig() {

        var dict ini.Dict

        sync_genie_config_loading_lock.Lock()

        sync_genie_config.keywords = make(map[string]SyncGenieKeywords)

        dict, err := ini.Load(*sync_genie_config_file)

        defer sync_genie_config_loading_lock.Unlock()

        if err != nil {
                em := fmt.Sprint("Config: failed to load config file", *sync_genie_config_file, "; got error: ", err)

                if sync_genie_config_loaded == true {
                        log.Println(em)
                        log.Println("Config: reload aborted")
                        return
                } else {
                        log.Fatalln(em)
                }
        }

        sync_genie_config_loaded = true

        sections := dict.GetSections()

        for _, section := range sections {

                // TODO replace goini to purge these hacks
                if section == "" {
                        continue
                }

                if section == "syncgenie" {
                        if match, e := dict.GetString(section, "watch_directory"); e != false {
                                sync_genie_config.watch_directory = &match
                                log.Println("Config: watch directory is:", *sync_genie_config.watch_directory)
                        }

                        sync_genie_config.watch_directory_poll = time.Duration(60)
                        if match, e := dict.GetInt(section, "watch_directory_poll"); e != false {
                                sync_genie_config.watch_directory_poll = time.Duration(match)
                        }

                        log.Println("Config: watch directory poll time is:", sync_genie_config.watch_directory_poll * time.Second)

                        sync_genie_config.watch_directory_max_depth = -1
                        if match, e := dict.GetInt(section, "watch_directory_max_depth"); e != false {
                                sync_genie_config.watch_directory_max_depth = match
                        }

                        log.Println("Config: watch directory depth level:", sync_genie_config.watch_directory_max_depth)

                        if match, e := dict.GetString(section, "run_when_done"); e != false {
                                sync_genie_config.run_when_done = match

                                log.Println("Config: command to run when a copy finishes:", sync_genie_config.run_when_done)
                        }

                        sync_genie_config.concurrent_copies = 1
                        if match, e := dict.GetInt(section, "concurrent_copies"); e != false {
                                sync_genie_config.concurrent_copies = match
                        }

                        log.Println("Config: copies that will run concurrently:", sync_genie_config.concurrent_copies)

                        sync_genie_config.verbose_listing = false
                        if match, e := dict.GetBool(section, "verbose_listing"); e != false {
                                sync_genie_config.verbose_listing = match
                                log.Println("Config: verbose file listing is enabled")
                        }

                        sync_genie_config.age_before_copy = time.Duration(60 * 5)
                        if match, e := dict.GetInt(section, "age_before_copy"); e != false {
                                sync_genie_config.age_before_copy = time.Duration(match)
                        }

                        log.Println("Config: will copy files older than", sync_genie_config.age_before_copy * time.Second)

                        continue
                }

                // destination
                destination, _ := dict.GetString(section, "destination")

                // keywords_dir

                var keywords_directories []string

                kw_dir, _ := dict.GetString(section, "keywords_directories")

                if kw_dir != "" {
                        var kws []string
                        var kwsf []string

                        kws = strings.Split(strings.TrimSpace(kw_dir), ",")

                        for _, v := range kws {
                                // TODO replace goini to purge these hacks
                                if v != "" {
                                        kwsf = append(kwsf, strings.TrimSpace(v))
                                }
                        }

                        keywords_directories = kwsf
                }


                // keywords
                keywords, _ := dict.GetString(section, "keywords")

                k_sections := strings.Split(strings.TrimSpace(keywords), "/")

                for _, v := range k_sections {

                        var m SyncGenieKeywords

                        k_subsec := strings.Split(strings.TrimSpace(v), ",")

                        var filtered []string

                        for _, f := range k_subsec {
                                // TODO replace goini to purge these hacks
                                if f != "" {
                                        filtered = append(filtered, strings.TrimSpace(f))
                                }
                        }

                        m.keywords = filtered
                        m.keywords_directories = keywords_directories
                        m.destination = destination

                        sync_genie_config.keywords[v] = m

                        log.Println("Config: added section", v, "with keywords:", filtered,
                                "; keyword dirs:", kw_dir,
                                "; destination:", destination)
                }
        }

        var keys []string

        for k := range sync_genie_config.keywords {
                keys = append(keys, k)
        }

        sort.Strings(keys)

        log.Println("Config: loaded, keyword sections:", strings.Join(keys, ", "))
}

func SyncGenieLoadHistory() {
        var hf *os.File
        var err error

        pwd, _ := os.Getwd()

        sync_genie_history_file = path.Join(pwd, "syncgenie-history.json")

        _, err = os.Stat(sync_genie_history_file)

        if err == nil {
                hf, err = os.Open(sync_genie_history_file)
        } else {
                hf, err = os.Create(sync_genie_history_file)
        }

        if err != nil {
                log.Println("History: unable to load history file", sync_genie_history_file)
                return
        }

        var b []byte

        b, err = ioutil.ReadAll(hf)

        err = json.Unmarshal(b, &sync_genie_history)

        log.Println("History: loaded", len(sync_genie_history), "entries")
}

func SyncGenieQueueCopy(fpath string, info os.FileInfo, err error) error {

        base := fpath

        if info.IsDir() == false {
                if sync_genie_config.verbose_listing == true {
                        log.Println("List: indexing", fpath)
                }
                base = filepath.Dir(fpath)
        }

        parts := strings.Split(filepath.Clean(base), string(os.PathSeparator))

        root_parts := strings.Split(filepath.Clean(*sync_genie_config.watch_directory), string(os.PathSeparator))

        if sync_genie_config.watch_directory_max_depth > -1 && len(parts) - len(root_parts) > sync_genie_config.watch_directory_max_depth {
                log.Println("List: Maximum watch directory depth,", sync_genie_config.watch_directory_max_depth, "reached, skipping children files in", base)
                return filepath.SkipDir
        }

        for _, v := range sync_genie_config.keywords {

                matched := false

                if info.IsDir() == true {
                        continue
                }

                for _, s := range v.keywords {

                        if m, _ := regexp.MatchString(strings.ToLower(s), strings.ToLower(filepath.Base(fpath))); m == false {
                                matched = false
                                break
                        } else {
                                matched = true

                                dmatched := false

                                if len(v.keywords_directories) > 0 {
                                        for _, d := range v.keywords_directories {

                                                for _, sd := range parts {

                                                        if dm, _ := regexp.MatchString(strings.ToLower(sd), strings.ToLower(d)); dm == true && sd != "" {

                                                                dmatched = true
                                                                break
                                                        }
                                                }
                                        }

                                        if matched && dmatched == false {
                                                matched = false
                                        }
                                }
                        }
                }

                if matched == true {
                        var new_copy_item SyncGenieCopyItem
                        new_copy_item.from = fpath
                        new_copy_item.to = path.Join(v.destination, filepath.Base(fpath))
                        s, _ := os.Stat(fpath)

                        if s.Size() == 0 {
				log.Println("Copy:", fpath, "is 0 bytes long, canceling copy")
                                return nil
                        }

                        for _, k := range sync_genie_history {
                                if k == new_copy_item.to {
                                        return nil
                                }
                        }

                        if s.ModTime().Add(sync_genie_config.age_before_copy * time.Second).Unix() > time.Now().Unix() {
                                return nil
                        }

                        new_copy_item.remaining = s.Size()

                        fi, err := os.Stat(new_copy_item.to)

                        if err == nil {
                                switch size := fi.Size(); {
                                case size == 0:
                                        log.Println("Copy:", new_copy_item.to, "is 0 bytes long, canceling copy")
                                        return nil
                                case size > 0 && size < s.Size():
                                        log.Println("Copy:", new_copy_item.to, "already exists but is smaller than the source by", s.Size() - size, "bytes; queuing to finish transfer")
					new_copy_item.remaining -= size
                                case size == s.Size():
                                        return nil
                                }

                        }

                        sync_genie_chan_new_files <-new_copy_item
                        log.Println("Copy: Queued new copy from", new_copy_item.from, "to", new_copy_item.to)
                }
        }

        return nil
}

func SyncGenieLister(ch chan int) {
        for {
                fp := *sync_genie_config.watch_directory
                if o, e := os.Stat(fp); e == nil && o.IsDir() {
                        now := time.Now()
                        filepath.Walk(fp, SyncGenieQueueCopy)
                        ch <- 1
                        log.Println("List: done listing; elapsed time", time.Since(now))
                } else {
                        log.Fatal("List: watch directory, '", fp, "', does not exist")
                }

                time.Sleep(sync_genie_config.watch_directory_poll * time.Second)
        }
}

func SyncGenieCopy(ch chan SyncGenieCopyProgress, new_file SyncGenieCopyItem) {
        var sf *os.File
        var df *os.File
        var err error

        sf, err = os.Open(new_file.from)
        if err != nil {
                log.Println("Copy: error: failed to open", new_file.from, "got error:", err)
                return
        }
        defer sf.Close()

        ss, err := os.Stat(new_file.from)

        if err != nil {
                log.Println("Copy: failed stat file to be copied, ", new_file.from, "; canceling copy")
                return
        }

	var ss_size = ss.Size()

        ds, err := os.Stat(new_file.to)

        var read_start int64 = 0
        var offset int64 = 0
        var wrote int64 = 0

        if err == nil {
                df, err = os.OpenFile(new_file.to, os.O_WRONLY, 0644)
                df.Seek(0, os.SEEK_SET)
                read_start = ds.Size()
                wrote = ds.Size()

        } else {
                df, err = os.Create(new_file.to)
        }

        if err != nil {
                log.Println("Copy: error: failed to open", new_file.to, "got error:", err)
                return
        }

        defer df.Close()

        buf := make([]byte, sync_genie_copy_buffer_size)

        sr := io.NewSectionReader(sf, read_start, ss.Size())

        progress_completion_states := [...]int{ 0, 0, 0, 0, 0 }

        for {
                read, e := sr.ReadAt(buf, offset)

                if read == 0 {
                        break
                }

                if e != nil && e != io.EOF {
                        log.Println("Copy: failed read at offset", offset, "; read start:", read_start, "; read", read, "bytes; on file", new_file.from, "; deferring copy; error:", e)
                        return
                }

                w, e := df.WriteAt(buf[0:read], read_start + offset)

                // log.Println("Copy: read", read, "bytes; wrote", w, "bytes", "; offset", offset)

                if int64(read) != int64(w) {
                        log.Println("Copy: error: failed to write at offset", offset, "; error:", e)
                        break
                }

                if e != nil {
                        log.Println("Copy: error: failed to write with error:", e)
                        break
                }

                wrote += int64(w)
                new_file.remaining -= int64(w)

		if ss_size - offset < sync_genie_copy_buffer_size {
			buf = make([]byte, ss_size - offset)
		}

                offset = offset + sync_genie_copy_buffer_size

                var completed int = int(float64(wrote) / float64(ss.Size()) * 100)

                if completed % 25 == 0 && progress_completion_states[int(math.Floor(float64(completed) / 25))] != 1 {
                        var progress SyncGenieCopyProgress
                        progress.file = filepath.Base(new_file.to)
                        progress.progress = completed
                        progress_completion_states[int(math.Floor(float64(completed) / 25))] = 1
                        ch <- progress
                }

                sync_genie_currently_copying[new_file.to] = wrote
        }


        switch size := wrote; {
        case size == 0:
                log.Println("Copy: error:", new_file.to, "copied 0 bytes, should have copied", new_file.remaining, "bytes")
        case new_file.remaining == 0:
                log.Println("Copy: done copying", new_file.from, "to", new_file.to, "(read", size, "bytes)")

                if sync_genie_config.run_when_done != "" {
                        filtered := sync_genie_config.run_when_done

                        filtered = strings.Replace(filtered, "{filename}", filepath.Base(new_file.to), -1)

                        parts := strings.Split(filtered, " ")

                        log.Println("Exec: running", parts)

                        c := exec.Command(parts[0], parts[1:]...)

                        err := c.Start()

                        if err != nil {
                                log.Println("Exec: failed to run command", parts)
                        }

                        err = c.Wait()

                        if err != nil {
                                log.Println("Exec: command,", filtered, "finished with error:", err)
                        }
                }
                delete(sync_genie_currently_copying, new_file.to)

                sync_genie_history = append(sync_genie_history, new_file.to)

                b, err := json.Marshal(&sync_genie_history)

                if err == nil {
                        e := ioutil.WriteFile(sync_genie_history_file, b, 0644)

                        if e == nil {
                                log.Println("History: updated history")
                        } else {
                                log.Println("History: failed to write history file,", sync_genie_history_file, "; error:", e)
                        }
                } else {
                        log.Println("History: failed to update history; error:", err)
                }

        }
}

func SyncGenieQueueUpCopy() {

        ch := make(chan SyncGenieCopyProgress)

        for {
                select {
                case new_file := <-sync_genie_chan_new_files:
                        sync_genie_currently_copying[new_file.to] = 0

                        go SyncGenieCopy(ch, new_file)
                case v, _ := <-ch:
                        log.Println("Copy: progress on", v.file, "is", v.progress, "percent done")
                }
        }
}

func main() {
        ch_signal := make(chan os.Signal, 1)

        signal.Notify(ch_signal, os.Interrupt)

        sync_genie_config = *new(SyncGenieConfig)

        sync_genie_config_loaded = false

        sync_genie_currently_copying = make(map[string]int64)

        pwd, _ := os.Getwd()

        sync_genie_config_file = flag.String("config", filepath.Join(pwd, "syncgenie.ini"), "config")

        flag.Parse()

        SyncGenieLoadConfig()

        SyncGenieLoadHistory()

        // 3 = the main goroutine, the file lister goroutine, and the copy sync goroutine
        runtime.GOMAXPROCS(sync_genie_config.concurrent_copies + 3)

        watcher, err := fsnotify.NewWatcher()
        if err != nil {
                log.Fatal(err)
        }
        err = watcher.Watch(*sync_genie_config_file)
        if err != nil {
                log.Fatal(err)
        }

        ch := make(chan int)
        sync_genie_chan_new_files = make(chan SyncGenieCopyItem)

        go SyncGenieLister(ch)

        go SyncGenieQueueUpCopy()

        for {
                select {
                case ev := <-watcher.Event:
                        if ev.IsModify() {
                                log.Println("Config: Reloading")
                                SyncGenieLoadConfig()
                        }
                case err := <-watcher.Error:
                        log.Println("Watcher: error:", err)
                case <-ch_signal:
                        if len(sync_genie_currently_copying) != 0 {
                                log.Println("Warning: currently copying files:", sync_genie_currently_copying)
                        }
                        log.Println("Quitting...")
                        os.Exit(1)
                case <-ch:

                }
        }
}

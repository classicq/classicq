package engine

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
)

// in memory
type packfile struct {
	name             string
	filepos, filelen int32
}

type pack struct {
	filename string
	handle   *os.File
	numfiles int
	files    []packfile
}

// on disk: "PACK", dirofs, dirlen; then 64 byte entries of name[56], filepos, filelen
const (
	dpackheaderSize = 12
	dpackfileSize   = 64
	dpackfileName   = 56
)

type searchpath struct {
	filename string
	pack     *pack // only one of filename / pack will be used
	next     *searchpath
}

var com_searchpaths *searchpath
var com_base_searchpaths *searchpath // without gamedirs

func FS_FileLength(f *os.File) int {
	pos, _ := f.Seek(0, io.SeekCurrent)
	end, _ := f.Seek(0, io.SeekEnd)
	f.Seek(pos, io.SeekStart)
	return int(end)
}

// Returns the open file and its length, or nil and -1.
func FS_FileOpenRead(path string) (*os.File, int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, -1
	}
	return f, FS_FileLength(f)
}

// The filename will be prefixed by com_basedir
func FS_WriteFile(filename string, data []byte) bool {
	name := strlcpy(com_basedir+"/"+filename, MAX_OSPATH)

	f, err := os.Create(name)
	if err != nil {
		FS_CreatePath(name)
		if f, err = os.Create(name); err != nil {
			return false
		}
	}

	Sys_Printf("FS_WriteFile: %s\n", name)
	f.Write(data)
	f.Close()
	return true
}

// Only used for CopyFile and download
func FS_CreatePath(path string) {
	if path == "" {
		return
	}
	for i := 1; i < len(path); i++ {
		if path[i] == '/' || (runtime.GOOS == "windows" && path[i] == '\\') {
			Sys_IO_Create_Directory(path[:i])
		}
	}
}

var file_from_pak int // global indicating file came from pack file ZOID
var file_from_gamedir int

func pakfileCompare(p packfile, name string) int {
	return strings.Compare(p.name, name)
}

// Finds the file in the search path.
// Sets com_filesize, com_netpath and returns the open file positioned at the
// data, or nil and -1.
func FS_FOpenFile(filename string) (*os.File, int) {
	file_from_pak = 0
	file_from_gamedir = 1
	com_filesize = -1
	com_netpath = ""

	// search through the path, one element at a time
	for search := com_searchpaths; search != nil; search = search.next {
		if search == com_base_searchpaths && com_searchpaths != com_base_searchpaths {
			file_from_gamedir = 0
		}

		// is the element a pak file?
		if search.pack != nil {
			// look through all the pak file elements
			pak := search.pack
			if i, found := slices.BinarySearchFunc(pak.files, filename, pakfileCompare); found {
				pakfile := &pak.files[i]
				if developer.value != 0 {
					Sys_Printf("PackFile: %s : %s\n", pak.filename, filename)
				}
				// open a new file on the pakfile
				f, err := os.Open(pak.filename)
				if err != nil {
					Sys_Error("Couldn't reopen %s", pak.filename)
				}
				f.Seek(int64(pakfile.filepos), io.SeekStart)
				com_filesize = int(pakfile.filelen)
				file_from_pak = 1
				com_netpath = strlcpy(fmt.Sprintf("%s#%d", pak.filename, i), MAX_OSPATH)
				return f, com_filesize
			}
		} else {
			com_netpath = strlcpy(search.filename+"/"+filename, MAX_OSPATH)
			f, err := os.Open(com_netpath)
			if err != nil {
				continue
			}
			if developer.value != 0 {
				Sys_Printf("FindFile: %s\n", com_netpath)
			}
			com_filesize = FS_FileLength(f)
			return f, com_filesize
		}
	}

	if developer.value != 0 {
		Sys_Printf("FindFile: can't find %s\n", filename)
	}
	return nil, -1
}

func FS_FileExists(filename string) int {
	// search through the path, one element at a time
	for search := com_searchpaths; search != nil; search = search.next {
		// is the element a pak file?
		if search.pack != nil {
			// look through all the pak file elements
			pak := search.pack
			if _, found := slices.BinarySearchFunc(pak.files, filename, pakfileCompare); found {
				return 1
			}
		} else {
			com_netpath = strlcpy(search.filename+"/"+filename, MAX_OSPATH)
			f, err := os.Open(com_netpath)
			if err != nil {
				continue
			}
			f.Close()
			return 1
		}
	}
	return 0
}

// Filenames are relative to the quake directory. Returns nil when the file
// is not found; C also appends a 0 byte, cstr() stands in for that.
func FS_LoadFile(path string) []byte {
	// look for it in the filesystem or pack files
	h, length := FS_FOpenFile(path)
	com_filesize = length
	if h == nil {
		return nil
	}

	buf := make([]byte, length)
	r, _ := io.ReadFull(h, buf)
	h.Close()
	if r != length {
		Sys_Error("FS_LoadFile: Error while reading file %s", path)
	}

	return buf
}

func FS_LoadZFile(path string) []byte {
	return FS_LoadFile(path)
}

func FS_LoadMallocFile(path string) []byte {
	return FS_LoadFile(path)
}

// Takes an explicit (not game tree related) path to a pak file.
// Loads the header and directory, adding the files at the beginning
// of the list so they override previous pack files.
func FS_LoadPackFile(packname string) *pack {
	packhandle, filelen := FS_FileOpenRead(packname)
	if filelen == -1 {
		return nil
	}

	var header [dpackheaderSize]byte
	io.ReadFull(packhandle, header[:])
	if filelen < dpackheaderSize || header[0] != 'P' || header[1] != 'A' || header[2] != 'C' || header[3] != 'K' {
		Sys_Error("%s is not a packfile", packname)
	}
	dirofs := int32(binary.LittleEndian.Uint32(header[4:]))
	dirlen := int32(binary.LittleEndian.Uint32(header[8:]))

	if dirofs < 0 || dirlen < 0 || int(dirofs) > filelen || int(dirofs)+int(dirlen) > filelen || dirofs+dirlen < dirofs {
		Sys_Error("%s is a malformed packfile", packname)
	}

	p := &pack{filename: packname, handle: packhandle}
	p.numfiles = int(dirlen) / dpackfileSize
	p.files = make([]packfile, p.numfiles)

	info := make([]byte, dirlen)
	packhandle.Seek(int64(dirofs), io.SeekStart)
	io.ReadFull(packhandle, info)

	// parse the directory
	for i := 0; i < p.numfiles; i++ {
		e := info[i*dpackfileSize : (i+1)*dpackfileSize]
		j := 0
		for j < dpackfileName && e[j] != 0 {
			j++
		}
		if j == dpackfileName {
			Sys_Error("%s is a malformed packfile", packname)
		}
		f := &p.files[i]
		f.name = string(e[:j])
		f.filepos = int32(binary.LittleEndian.Uint32(e[dpackfileName:]))
		f.filelen = int32(binary.LittleEndian.Uint32(e[dpackfileName+4:]))
		if f.filepos < 0 || f.filelen < 0 || int(f.filepos) > filelen || int(f.filepos)+int(f.filelen) > filelen || f.filepos+f.filelen < f.filepos {
			Sys_Error("%s is a malformed packfile", packname)
		}
	}

	// Sort the entries by name to make it easier to search
	slices.SortFunc(p.files, func(a, b packfile) int { return strings.Compare(a.name, b.name) })

	return p
}

func FS_FreePackFile(p *pack) {
	p.handle.Close()
}

func FS_FreeSearchPaths(searchpaths *searchpath) {
	for t := searchpaths; t != nil; t = t.next {
		if t.pack != nil {
			FS_FreePackFile(t.pack)
		}
	}
}

var fs_basedirs []string

// Sets com_gamedir, adds the directory to the head of the path, then loads
// and adds pak1.pak pak2.pak ...
func FS_AddGameDirectory_NoReally(dir string) {
	if len(dir)+1 >= MAX_OSPATH {
		Sys_Error("Game directory path too long\n")
		return
	}

	p := strings.LastIndexByte(dir, '/')
	if p < 0 {
		return
	}

	if len(dir)-p+1 >= MAX_QPATH {
		Sys_Error("Game directory path too long\n")
		return
	}

	com_gamedirfile = dir[p+1:]
	com_gamedir = dir

	for search := com_searchpaths; search != nil; search = search.next {
		// Path already exists
		if search.filename == dir {
			return
		}
	}

	// add the directory to the search path
	firstsearch := &searchpath{filename: dir, next: com_searchpaths}

	// add any pak files in the format pak0.pak pak1.pak, ...
	for i := 0; ; i++ {
		pakfile := strlcpy(fmt.Sprintf("%s/pak%d.pak", dir, i), MAX_OSPATH)
		pak := FS_LoadPackFile(pakfile)
		if pak == nil {
			break
		}
		firstsearch = &searchpath{filename: pakfile, pack: pak, next: firstsearch}
	}

	// classicq.pak (engine bundle)
	pakfile := strlcpy(dir+"/classicq.pak", MAX_OSPATH)
	if pak := FS_LoadPackFile(pakfile); pak != nil {
		firstsearch = &searchpath{filename: pakfile, pack: pak, next: firstsearch}
	}

	com_searchpaths = firstsearch
}

func FS_AddGameDirectory(dir string) {
	for _, base := range fs_basedirs {
		FS_AddGameDirectory_NoReally(va("%s/%s", base, dir))
	}
}

// Sets the gamedir and path to a different directory.
func FS_SetGamedir(dir string) {
	if strings.Contains(dir, "..") || strings.Contains(dir, "/") || strings.Contains(dir, "\\") || strings.Contains(dir, ":") {
		Com_Printf("Gamedir should be a single filename, not a path\n")
		return
	}

	if com_gamedirfile == dir {
		return // still the same
	}

	// free up any current game dir info
	if com_searchpaths != com_base_searchpaths {
		search := com_searchpaths
		for search.next != com_base_searchpaths {
			search = search.next
		}
		search.next = nil
		FS_FreeSearchPaths(com_searchpaths)
		com_searchpaths = com_base_searchpaths
	}

	FS_AddGameDirectory(dir)
}

var (
	ro_data_path     string
	user_data_path   string
	legacy_data_path string
)

func FS_CheckSubpathExist(path, subpath string) bool {
	return Sys_IO_Path_Exists(path + "/" + subpath)
}

func FS_InitFilesystem() {
	ro_data_path = Sys_GetRODataPath()
	user_data_path = Sys_GetUserDataPath()
	legacy_data_path = Sys_GetLegacyDataPath()

	preferlegacy := false
	legacywritable := false

	if legacy_data_path != "" {
		if Sys_IO_Path_Exists(legacy_data_path) && (FS_CheckSubpathExist(legacy_data_path, "id1") || FS_CheckSubpathExist(legacy_data_path, "classicq") || FS_CheckSubpathExist(legacy_data_path, "qw")) {
			legacywritable = Sys_IO_Path_Writable(legacy_data_path)
			if legacywritable {
				preferlegacy = FS_CheckSubpathExist(legacy_data_path, "classicq/temp") // That directory is created by MT_Init()
			}
		} else {
			legacy_data_path = ""
		}
	}

	userwritable := false

	if user_data_path != "" {
		Sys_IO_Create_Directory(user_data_path)

		if Sys_IO_Path_Exists(user_data_path) {
			userwritable = Sys_IO_Path_Writable(user_data_path)
		} else {
			user_data_path = ""
		}
	}

	if !legacywritable && !userwritable {
		orNone := func(s string) string {
			if s == "" {
				return "<none>"
			}
			return s
		}
		Sys_Error("Nowhere to write data. Check that either \"%s\" or \"%s\" is writable.\n", orNone(legacy_data_path), orNone(user_data_path))
	}

	fs_basedirs = nil

	if ro_data_path != "" {
		fs_basedirs = append(fs_basedirs, ro_data_path)
	}

	if user_data_path != "" && preferlegacy {
		fs_basedirs = append(fs_basedirs, user_data_path)
	}

	if legacy_data_path != "" {
		fs_basedirs = append(fs_basedirs, legacy_data_path)
	}

	if user_data_path != "" && !preferlegacy {
		fs_basedirs = append(fs_basedirs, user_data_path)
	}

	// -basedir has been a no-op for a long time; C adds the user path again.
	if i := COM_CheckParm("-basedir"); i != 0 && i < com_argc-1 && user_data_path != "" {
		com_basedir = strings.ReplaceAll(com_basedir, "\\", "/")
		fs_basedirs = append(fs_basedirs, user_data_path)
	}

	com_basedir = strlcpy(fs_basedirs[len(fs_basedirs)-1], MAX_OSPATH)

	FS_AddGameDirectory("id1")
	FS_AddGameDirectory("classicq")
	FS_AddGameDirectory("qw")

	if n := len(com_basedir); n > 0 && com_basedir[n-1] == '/' {
		com_basedir = com_basedir[:n-1]
	}

	// any set gamedirs will be freed up to here
	com_base_searchpaths = com_searchpaths

	// the user might want to override default game directory
	i := COM_CheckParm("-game")
	if i == 0 {
		i = COM_CheckParm("+gamedir")
	}
	if i != 0 && i < com_argc-1 {
		FS_SetGamedir(com_argv[i+1])
	}
}

func FS_ShutdownFilesystem() {
	FS_FreeSearchPaths(com_searchpaths)
	com_searchpaths = nil
}

func FS_Path_f() {
	Com_Printf("Current search path:\n")
	for s := com_searchpaths; s != nil; s = s.next {
		if s == com_base_searchpaths {
			Com_Printf("----------\n")
		}
		if s.pack != nil {
			Com_Printf("%s (%d files)\n", s.pack.filename, s.pack.numfiles)
		} else {
			Com_Printf("%s\n", s.filename)
		}
	}
}

func FS_Init() {
	Cmd_AddCommand("path", FS_Path_f)
}

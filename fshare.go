package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const SPATH = "./_SharedFiles/"
const DPATH = "./_Downloads/"
const CHUNCK_SIZE = 8 * 1024 // 8KB for the chunck size
const HASH_SIZE = 32
const MAX_NB_CHUNK = CHUNCK_SIZE / HASH_SIZE

type FileRecord struct {
	Name     string
	NbChunk  int
	MetaFile []string
	MetaHash string
}

func ScanFile(fname string) (*FileRecord, error) {

	fr := &FileRecord{Name: fname}

	f, err := os.Open(SPATH + fname)

	// If there is no path just ignore it
	if err != nil {

		return nil, err
	}

	defer f.Close()

	buf := make([]byte, CHUNCK_SIZE)

	tot := -1

	r := bufio.NewReader(f)

	hexs := make([]string, 0)
	concats := make([]byte, 0)

	for {

		n, _ := r.Read(buf)

		h := sha256.New()
		tot += 1

		if n == 0 {

			if tot > MAX_NB_CHUNK {
				return nil, errors.New("File too large to be indexed")
			}

			// store the information

			fr.NbChunk = tot
			fr.MetaFile = hexs

			metaBytes, err := hex.DecodeString(strings.Join(hexs, ""))
			checkError(err, true)
			h.Write(metaBytes)

			fr.MetaHash = hex.EncodeToString(h.Sum(nil))

			fmt.Println("MetaHash = ", fr.MetaHash)

			return fr, nil

		} else {
			h.Write(buf[0:n])
			b := h.Sum(nil)

			// If we need bytes uncomment this
			concats = append(concats, b...)
			hexs = append(hexs, hex.EncodeToString(b))

		}
	}
}

// When requesting a chunk, this method check if it is in its possession
// return  the index where it found it on the files record and which chunck
// (-1, -1) if no chunck corresponds
// (x, -1) if the metahash match the request of entry x in myGossiper.file
// (x , y) if the entry y of the Metafile x match
func chunkSeek(hash []byte, myGossiper *Gossiper) (int, int) {

	request := hex.EncodeToString(hash)

	for i, f := range myGossiper.safeFiles.files {

		if f.MetaHash == request {
			return i, -1
		}

		for j, chunk := range f.MetaFile {
			if chunk == request {
				return i, j
			}
		}
	}

	return -1, -1

}

func getDataAndHash(i int, j int, myGossiper *Gossiper) ([]byte, []byte) {

	var data []byte
	var hash []byte

	// If the meta file is requested
	if j == -1 {
		// Meta file is the deta
		fmt.Println(i)
		data, err := hex.DecodeString(strings.Join(myGossiper.safeFiles.files[i].MetaFile, ""))
		checkError(err, true)

		hash, err = hex.DecodeString(myGossiper.safeFiles.files[i].MetaHash)

		return data, hash

	}

	if j >= myGossiper.safeFiles.files[i].NbChunk {
		fmt.Println("Error : Chunk bigger than NbChunk was requested (fshare.go l.129)")
		os.Exit(1)
	}

	name := myGossiper.safeFiles.files[i].Name

	f, err := os.Open(SPATH + name)
	checkError(err, true)
	defer f.Close()

	// fetch the data in file

	_, err = f.Seek(int64(j*CHUNCK_SIZE), 0)

	checkError(err, true)

	buf := make([]byte, CHUNCK_SIZE)

	n, err := f.Read(buf)
	checkError(err, true)

	// Sanitary CcheckError
	h := sha256.New()
	h.Write(buf[0:n])
	hash = h.Sum(nil)
	sanitaryCheck, _ := hex.DecodeString(myGossiper.safeFiles.files[i].MetaFile[j])
	if !bytes.Equal(hash, sanitaryCheck) {
		fmt.Println("Sanitary Check failed! (l.137 in fshare.go)")
		//os.Exit(1)
	}
	data = buf[0:n]

	return data, hash
}

func EqualityCheckRecievedData(HashValue []byte, data []byte) bool {
	h := sha256.New()
	h.Write(data)

	return bytes.Equal(HashValue, h.Sum(nil))

}

func addFileRecord(fr *FileRecord, myGossiper *Gossiper) error {
	for _, f := range myGossiper.safeFiles.files {
		// Duplicate record
		if f.MetaHash == fr.MetaHash {
			fmt.Println("Duplicates")
			return errors.New(" content already indexed (same content as " + fr.Name + ")")
		}
	}

	myGossiper.safeFiles.files = append(myGossiper.safeFiles.files, fr)
	return nil
}

func storeFile(fname string, request []byte, myGossiper *Gossiper, i int) {

	// If we know already where to seek, we don't need seek where to seek
	j := -1
	if request != nil {
		i, j = chunkSeek(request, myGossiper)
	}

	if i != -1 && j == -1 {

		sfname := myGossiper.safeFiles.files[i].Name

		src, err := os.Open(SPATH + sfname)
		checkError(err, true)
		defer src.Close()

		dst, err := os.Create(DPATH + fname)
		checkError(err, true)
		defer dst.Close()

		_, err = io.Copy(dst, src)
		checkError(err, true)
	}

}

func WriteChunk(fname string, data []byte) {
	if _, err := os.Stat(SPATH + fname); os.IsNotExist(err) {
		os.Create(SPATH + fname)
	}

	f, err := os.OpenFile(SPATH+fname, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println("HEY")
	}

	defer f.Close()

	_, err = f.Write(data)

	if err != nil {
		fmt.Println("HEY")
	}

}

// If we want to store chunk somewhere else than in the memory of the prgramm

/* For init


func saveFile(fr *FileRecord) {
	// Put the meta hash as the name of the file
	json := simplejson.New()
	json.Set("Name", fr.Name)
	json.Set("MetaHash", fr.MetaHash)
	json.Set("MetaFile", fr.MetaFile)
	json.Set("Size", fr.Size)

}



func loadIndexedFiles() {

	if _, err := os.Stat("./_SharedFiles/.Meta"); os.IsNotExist(err) {
		os.Mkdir("./_SharedFiles/.Meta", os.ModePerm)
		return
	}

	files, err := ioutil.ReadDir("./_SharedFiles/.Meta")
	checkError(err)

	for _, file := range files {

	}
}
*/

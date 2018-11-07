package main

import(
  "os"
  "fmt"
  "bufio"
  "crypto/sha256"
  "strings"
  "encoding/hex"
)

const FPATH = "./_SharedFiles/"
const CHUNCK_SIZE = 8 * 1024 // 8KB for the chunck size

type FileRecord struct {
  Name string
  Size int
  MetaFile []string
  MetaHash string
}

type DataRequest struct{
 Origin string
 Destination string
 HopLimit uint32
 HashValue []byte
}


type DataReply struct{
 Origin string
 Destination string
 HopLimit uint32
 HashValue []byte
 Data []byte
}

func ScanFile(fname string) *FileRecord{

  f, err := os.Open(FPATH + fname)
  checkError(err)
  defer f.Close()

  fr := &FileRecord{Name: fname}

  buf := make([]byte, CHUNCK_SIZE)
  checkError(err)

  tot := 0

  r := bufio.NewReader(f)

  hexs := make([]string, 0)
  concats := make([]byte, 0)

  for  {

    n, _ := r.Read(buf)

    h := sha256.New()
    tot += n

    if n == 0 {

      // store the informations

      fr.Size = tot
      fr.MetaFile = hexs

      metaBytes, err := hex.DecodeString(strings.Join(hexs, ""))
      checkError(err)
      h.Write(metaBytes)

      fr.MetaHash = hex.EncodeToString(h.Sum(nil))

      //fmt.Println("MetaHash = ", fr.MetaHash)


      return fr


    }else{
      h.Write(buf[0:n])
      b := h.Sum(nil)
      fmt.Println("n = ", n , " sha = ", b)

      // If we need bytes uncomment this
      concats = append(concats, b...)
      hexs = append(hexs, hex.EncodeToString(b))


    }
  }
}

func saveFile(*FileRecord){
  // Put the meta hash as the name of the file 
}

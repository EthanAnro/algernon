// Package ollama provides Lua functions for communicting with a local Ollama server
package ollama

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"github.com/xyproto/algernon/lua/convert"
	lua "github.com/xyproto/gopher-lua"
	"github.com/xyproto/ollamaclient/v2"
)

const (
	// Class is an identifier for the OllamaClient class in Lua
	Class = "OllamaClient"

	defaultModel  = "tinyllama"
	defaultPrompt = "Generate a haiku about the poet Algernon"
)

var mut sync.RWMutex

// Get the first argument, "self", and cast it from userdata to a library (which is really a hash map).
func checkOllamaClient(L *lua.LState) *ollamaclient.Config {
	ud := L.CheckUserData(1)
	if ollama, ok := ud.Value.(*ollamaclient.Config); ok {
		return ollama
	}
	L.ArgError(1, "ollamaclient.Config expected")
	return nil
}

// ollamaPullIfNeeded will download the active model, if it's missing
// it takes an optional "verbose" argument that is bool.
func ollamaPullIfNeeded(L *lua.LState) int {
	oc := checkOllamaClient(L) // arg 1
	// Pull the model, in a verbose way
	err := oc.PullIfNeeded(true)
	if err != nil {
		log.Error(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}
	return 0 // number of results
}

// ollamaHas will check if the given model has been downloaded
func ollamaHas(L *lua.LState) int {
	oc := checkOllamaClient(L) // arg 1
	// Check if the given model name has already been downloaded
	modelName := L.ToString(2) // arg 2
	found, err := oc.Has(modelName)
	if err != nil {
		log.Error(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}
	L.Push(lua.LBool(found))
	return 1 // number of results
}

// ollamaList will list all downloaded models, if possible
func ollamaList(L *lua.LState) int {
	oc := checkOllamaClient(L) // arg 1
	downloadedModels, _, _, err := oc.List()
	if err != nil {
		log.Error(err)
		L.Push(convert.Strings2table(L, []string{}))
		return 1 // number of results
	}
	L.Push(convert.Strings2table(L, downloadedModels))
	return 1 // number of results
}

// ollamaSizeInBytes will check the size on disk for the given model, if possible
func ollamaSizeInBytes(L *lua.LState) int {
	oc := checkOllamaClient(L) // arg 1
	top := L.GetTop()
	mut.RLock()
	modelName := oc.ModelName
	mut.RUnlock()
	if top > 1 {
		modelName = L.ToString(2)
	}
	size, err := oc.SizeOf(modelName) // get the size of the given model name
	if err != nil {
		log.Error(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}
	L.Push(lua.LNumber(size))
	return 1 // number of results
}

// ollamaSize checks the size on disk for the given model, if possible,
// and returns the size as a human-friendly string using the humanize package.
func ollamaSize(L *lua.LState) int {
	oc := checkOllamaClient(L) // Assume this is a function that checks for an Ollama client instance
	top := L.GetTop()
	mut.RLock()
	modelName := oc.ModelName
	mut.RUnlock()
	if top > 1 {
		modelName = L.ToString(2)
	}
	size, err := oc.SizeOf(modelName) // Assume this gets the size of the given model name in bytes
	if err != nil {
		log.Println(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}

	// Use humanize package to format size
	sizeStr := humanize.Bytes(uint64(size))
	L.Push(lua.LString(sizeStr))
	return 1 // number of results
}

// ollamaModel sets or gets the given model name, but does not pull anything
func ollamaModel(L *lua.LState) int {
	oc := checkOllamaClient(L) // Assume this is a function that checks for an Ollama client instance
	top := L.GetTop()
	if top < 2 {
		// Return the current model name if no model is passed in
		mut.RLock()
		model := oc.ModelName
		mut.RUnlock()
		L.Push(lua.LString(model))
		return 1 // number of results
	}
	modelName := strings.TrimSpace(L.ToString(2))
	L.Push(lua.LString(modelName))
	mut.Lock()
	oc.ModelName = modelName
	mut.Unlock()
	return 1 // number of results
}

func ollamaGenerateOutput(L *lua.LState) int {
	mut.Lock()
	defer mut.Unlock()
	oc := checkOllamaClient(L) // arg 1
	prompt := defaultPrompt
	top := L.GetTop()
	if top > 2 {
		prompt = L.ToString(2)       // arg 2
		oc.ModelName = L.ToString(3) // arg 3
		err := oc.PullIfNeeded(true)
		if err != nil {
			log.Error(err)
			L.Push(lua.LString(err.Error()))
			return 1 // number of results
		}
	} else if top > 1 {
		prompt = L.ToString(2) // arg 2
	}
	oc.SetReproducible()
	output, err := oc.GetOutput(prompt)
	if err != nil {
		log.Error(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}
	L.Push(lua.LString(strings.TrimPrefix(output, " ")))
	return 1 // number of results
}

// Use ollama to get a []float64 representation of a given prompt.
// This can be used to ie. find the distance between two strings.
func ollamaEmbeddings(L *lua.LState) int {
	mut.Lock()
	defer mut.Unlock()
	oc := checkOllamaClient(L) // arg 1
	prompt := defaultPrompt
	top := L.GetTop()
	if top > 2 {
		prompt = L.ToString(2)       // arg 2
		oc.ModelName = L.ToString(3) // arg 3
		err := oc.PullIfNeeded(true)
		if err != nil {
			log.Error(err)
			L.Push(lua.LString(err.Error()))
			return 1 // number of results
		}
	} else if top > 1 {
		prompt = L.ToString(2) // arg 2
	}
	oc.SetReproducible()
	var err error
	var floats []float64
	floats, err = oc.Embeddings(prompt)
	if err != nil {
		log.Error(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}

	// Push the floats as Lua numbers
	tbl := L.NewTable()
	for i, f := range floats {
		index := fmt.Sprintf("%d", i+1) // Convert index to string for Lua table
		L.SetField(tbl, index, lua.LNumber(f))
	}
	L.Push(tbl)
	return 1 // number of results
}

func ollamaGenerateOutputCreative(L *lua.LState) int {
	mut.Lock()
	defer mut.Unlock()
	oc := checkOllamaClient(L) // arg 1
	prompt := defaultPrompt
	top := L.GetTop()
	if top > 2 {
		prompt = L.ToString(2)       // arg 2
		oc.ModelName = L.ToString(3) // arg 3
		err := oc.PullIfNeeded(true)
		if err != nil {
			log.Error(err)
			L.Push(lua.LString(err.Error()))
			return 1 // number of results
		}
	} else if top > 1 {
		prompt = L.ToString(2) // arg 2
	}
	oc.SetRandom()
	output, err := oc.GetOutput(prompt)
	if err != nil {
		log.Error(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}
	L.Push(lua.LString(output))
	return 1 // number of results
}

func constructOllamaClient(L *lua.LState) (*lua.LUserData, error) {
	oc := ollamaclient.New()
	top := L.GetTop()
	if top > 1 { // given two strings, the model and host address
		oc.ModelName = L.ToString(1)
		oc.ServerAddr = L.ToString(2)
	} else if top > 0 { // given one string, the model
		oc.ModelName = L.ToString(1)
	} else {
		oc.ModelName = defaultModel
	}
	// Create a new userdata struct
	ud := L.NewUserData()
	ud.Value = oc
	L.SetMetatable(ud, L.GetTypeMetatable(Class))
	return ud, nil
}

// The hash map methods that are to be registered
var ollamaMethods = map[string]lua.LGFunction{
	"ask":        ollamaGenerateOutput,
	"bytesize":   ollamaSizeInBytes,
	"creative":   ollamaGenerateOutputCreative,
	"has":        ollamaHas,
	"list":       ollamaList,
	"model":      ollamaModel, // set or get the current model, but don't pull anything
	"pull":       ollamaPullIfNeeded,
	"size":       ollamaSize,
	"embeddings": ollamaEmbeddings, // get a []float64 representation of a given prompt
}

func askOllama(L *lua.LState) int {
	prompt := defaultPrompt
	model := defaultModel
	top := L.GetTop()
	if top > 1 {
		prompt = L.ToString(1)
		model = L.ToString(2)
	} else if top > 0 {
		prompt = L.ToString(1)
	}
	oc := ollamaclient.New()
	oc.ModelName = model
	// Pull the model, in a verbose way
	err := oc.PullIfNeeded(true)
	if err != nil {
		log.Error(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}
	output, err := oc.GetOutput(prompt)
	if err != nil {
		log.Error(err)
		L.Push(lua.LString(err.Error()))
		return 1 // number of results
	}
	L.Push(lua.LString(output))
	return 1 // number of results
}

// distance calculates the cosine similarity between two embeddings (Lua tables of floats).
func distance(L *lua.LState) int {
	// Check and get the first table argument
	tbl1 := L.CheckTable(1)
	// Check and get the second table argument
	tbl2 := L.CheckTable(2)

	// Convert Lua tables to slices of float64
	slice1, err1 := tableToFloatSlice(tbl1)
	if err1 != nil {
		L.Push(lua.LString(err1.Error()))
		return 1 // number of results (error message)
	}
	slice2, err2 := tableToFloatSlice(tbl2)
	if err2 != nil {
		L.Push(lua.LString(err2.Error()))
		return 1 // number of results (error message)
	}

	// Calculate the cosine similarity
	if len(slice1) != len(slice2) {
		L.Push(lua.LString("error: embeddings must be of the same length"))
		return 1 // number of results (error message)
	}

	dotProduct := 0.0
	normA := 0.0
	normB := 0.0
	for i := range slice1 {
		dotProduct += slice1[i] * slice2[i]
		normA += slice1[i] * slice1[i]
		normB += slice2[i] * slice2[i]
	}
	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA == 0.0 || normB == 0.0 {
		L.Push(lua.LString("error: one or both vectors are zero vectors"))
		return 1 // number of results
	}

	cosineSimilarity := dotProduct / (normA * normB)

	// Cosine similarity ranges from -1 to 1, higher values mean more similarity
	// We convert it to a distance measure that ranges from 0 to 2
	cosineDistance := 1 - cosineSimilarity
	L.Push(lua.LNumber(cosineDistance))
	return 1 // number of results (distance)
}

// Load makes functions related Ollama clients available to the given Lua state
func Load(L *lua.LState) {
	// Register the OllamaClient class and the methods that belongs with it.
	mt := L.NewTypeMetatable(Class)
	mt.RawSetH(lua.LString("__index"), mt)
	L.SetFuncs(mt, ollamaMethods)

	// The constructor for new Libraries takes only an optional id
	L.SetGlobal("OllamaClient", L.NewFunction(func(L *lua.LState) int {
		// Construct a new OllamaClient
		userdata, err := constructOllamaClient(L)
		if err != nil {
			log.Error(err)
			L.Push(lua.LString(err.Error()))
			return 1 // Number of returned values
		}
		// Return the Lua OllamaClient object
		L.Push(userdata)
		return 1 // number of results
	}))

	L.SetGlobal("ollama", L.NewFunction(askOllama))
	L.SetGlobal("edistance", L.NewFunction(distance))
}

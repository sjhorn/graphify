package julia

// #include "parser.h"
// const TSLanguage *tree_sitter_julia(void);
import "C"
import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// GetLanguage returns the julia language definition for tree-sitter
func GetLanguage() *tree_sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_julia())
	return tree_sitter.NewLanguage(ptr)
}

Modify Decode to detect if a struct pointer or a slice pointer has been
passed. If a slice call readlines. If a struct call readline

Having done the above allow for multiple types of structs to be parsed by copying the sync.Hash method used in go-fixedwidth

This will also optimise for the use case of "load more than one file"


Consider allowing a skipheaders property. Right now, setting columns manually sets headersParsed to true. What happens if a user wants to 
skip the headers even though they have set the columsn - they need to be enable to set skip headers to false


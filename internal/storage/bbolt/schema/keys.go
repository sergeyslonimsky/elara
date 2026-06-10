package schema

const keySep = byte(0x00)

// schemaKey encodes (namespace, pathPattern) as the bucket key.
func schemaKey(namespace, pathPattern string) []byte {
	return []byte(namespace + string(keySep) + pathPattern)
}

// schemaKeyPrefix returns the prefix used to scan all schemas for a namespace.
func schemaKeyPrefix(namespace string) []byte {
	return []byte(namespace + string(keySep))
}

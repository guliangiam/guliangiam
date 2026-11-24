package cred

// HashProvider is the interface for password hashing algorithms.
type HashProvider interface {
	// Hash generates a hash from the plaintext password.
	Hash(password string) (string, error)
	// Verify checks if the password matches the hash.
	Verify(password, hash string) (bool, error)
	// Algorithm returns the algorithm identifier.
	Algorithm() string
}

// Algorithm constants for password hashing.
const (
	AlgoArgon2id = "argon2id"
	AlgoBcrypt   = "bcrypt"
	AlgoPBKDF2   = "pbkdf2"
	AlgoSHA256   = "sha256"
	AlgoSHA512   = "sha512"
	AlgoMD5      = "md5"
	AlgoPlain    = "plain"
)

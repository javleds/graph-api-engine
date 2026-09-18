package identifier

type Generator interface{ NewID() (string, error) }

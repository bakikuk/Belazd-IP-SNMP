package accounting

// IPv4DatagramLength is L3 bytes to bill (IPv4 Total Length when present).
func IPv4DatagramLength(totalLen uint16, capturedLen int) uint64 {
	if totalLen >= 20 {
		return uint64(totalLen)
	}
	if capturedLen > 0 {
		return uint64(capturedLen)
	}
	return 0
}

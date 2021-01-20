package acserver

var (
	TCPBytesRead        int
	TCPBytesWritten     int
	TCPMessagesReceived int
	TCPMessagesSent     int
	UDPBytesRead        int
	UDPBytesWritten     int
	UDPMessagesReceived int
	UDPMessagesSent     int
)

func clearStatistics() {
	TCPBytesRead = 0
	TCPBytesWritten = 0
	UDPBytesRead = 0
	UDPBytesWritten = 0
	TCPMessagesReceived = 0
	TCPMessagesSent = 0
	UDPMessagesReceived = 0
	UDPMessagesSent = 0
}

func printStatistics(logger Logger) {
	logger.Infof("Statistics: UDP: %d bytes read, %d bytes written. TCP: %d bytes read, %d bytes written.", UDPBytesRead, UDPBytesWritten, TCPBytesRead, TCPBytesWritten)
	logger.Infof("Statistics: UDP: %d messages received, %d messages sent. TCP: %d messages received, %d messages sent.", UDPMessagesReceived, UDPMessagesSent, TCPMessagesReceived, TCPMessagesSent)
	logger.Infof("Statistics: UDP: %d bytes avg receive size, %d bytes avg send size. TCP: %d bytes avg receive size, %d bytes avg send size", UDPBytesRead/UDPMessagesReceived, UDPBytesWritten/UDPMessagesSent, TCPBytesRead/TCPMessagesReceived, TCPBytesWritten/TCPMessagesSent)
}

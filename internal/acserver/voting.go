package acserver

import (
	"fmt"
	"net"
	"time"
)

type VoteNextSessionHandler struct {
	votingManager *VotingManager
}

func NewVoteNextSessionHandler(manager *VotingManager) *VoteNextSessionHandler {
	return &VoteNextSessionHandler{votingManager: manager}
}

func (vnh VoteNextSessionHandler) OnMessage(conn net.Conn, p *Packet) error {
	var forOrAgainst uint8

	p.Read(&forOrAgainst)

	return vnh.votingManager.SetVote(vnh.MessageType(), forOrAgainst, 0, conn)
}

func (vnh VoteNextSessionHandler) MessageType() MessageType {
	return TCPMessageVoteNextSession
}

type VoteRestartSessionHandler struct {
	votingManager *VotingManager
}

func NewVoteRestartSessionHandler(manager *VotingManager) *VoteRestartSessionHandler {
	return &VoteRestartSessionHandler{votingManager: manager}
}

func (vrh VoteRestartSessionHandler) OnMessage(conn net.Conn, p *Packet) error {
	var forOrAgainst uint8

	p.Read(&forOrAgainst)

	return vrh.votingManager.SetVote(vrh.MessageType(), forOrAgainst, 0, conn)
}

func (vrh VoteRestartSessionHandler) MessageType() MessageType {
	return TCPMessageVoteRestartSession
}

type VoteKickHandler struct {
	votingManager *VotingManager
}

func NewVoteKickHandler(manager *VotingManager) *VoteKickHandler {
	return &VoteKickHandler{votingManager: manager}
}

func (vkh VoteKickHandler) OnMessage(conn net.Conn, p *Packet) error {
	var carIDToKick CarID
	var forOrAgainst uint8

	p.Read(&carIDToKick)
	p.Read(&forOrAgainst)

	return vkh.votingManager.SetVote(vkh.MessageType(), forOrAgainst, carIDToKick, conn)
}

func (vkh VoteKickHandler) MessageType() MessageType {
	return TCPMessageVoteKick
}

type VotingManager struct {
	state          *ServerState
	sessionManager *SessionManager
	logger         Logger

	currentVote *Vote
}

func NewVotingManager(state *ServerState, sessionManager *SessionManager, logger Logger) *VotingManager {
	return &VotingManager{
		state:          state,
		sessionManager: sessionManager,
		logger:         logger,
	}
}

type Vote struct {
	VoteType         MessageType
	NumVotes         uint8
	VotedIDs         map[CarID]bool
	NumConnected     uint8
	ConnectedClients uint8
	KickID           CarID
	VoteFinishTime   uint32
}

func (vm *VotingManager) BroadcastVote(id CarID) {
	if vm.currentVote == nil {
		return
	}

	p := NewPacket(nil)

	p.Write(vm.currentVote.VoteType)
	p.Write(vm.currentVote.KickID)                                                     // car ID to kick, 0 if other vote
	p.Write(vm.currentVote.NumConnected)                                               // num connected clients
	p.Write(vm.currentVote.NumVotes)                                                   // num votes
	p.Write(vm.currentVote.VoteFinishTime - uint32(vm.state.CurrentTimeMillisecond())) // vote time remaining
	p.Write(id)                                                                        // carID voted
	p.Write(uint8(0x01))                                                               // either 1 or 0, never anything else

	vm.logger.Debugf("Broadcasting vote packet")

	vm.state.BroadcastAllTCP(p)
}

func (vm *VotingManager) SetVote(voteType MessageType, forOrAgainst uint8, kickID CarID, conn net.Conn) error {
	entrant, err := vm.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	if vm.currentVote == nil {
		vm.logger.Debugf("Vote started type: 0x%x, num votes: %d", voteType, forOrAgainst)

		vm.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s has started a vote! %d seconds remaining", entrant.Driver.Name, vm.state.serverConfig.VoteDuration), false)

		// start new vote
		vm.currentVote = &Vote{
			VoteType:       voteType,
			VoteFinishTime: uint32(vm.state.CurrentTimeMillisecond() + int64(vm.state.serverConfig.VoteDuration*1000)),
			NumVotes:       forOrAgainst,
			NumConnected:   uint8(vm.state.entryList.NumConnected()),
			KickID:         kickID,
			VotedIDs:       make(map[CarID]bool),
		}

		go func() {
			time.Sleep(time.Second * time.Duration(vm.state.serverConfig.VoteDuration))

			vm.StepVote(0xffffffff)
		}()
	} else {
		if _, ok := vm.currentVote.VotedIDs[entrant.CarID]; ok {
			vm.logger.Debugf("%s attempted to vote multiple times, ignored", entrant.Driver.Name)
			return nil
		}

		vm.logger.Debugf("%s voted %d in 0x%x", entrant.Driver.Name, forOrAgainst, voteType)

		if voteType == vm.currentVote.VoteType {
			vm.currentVote.NumVotes += forOrAgainst
		} else {
			err := vm.state.SendChat(ServerCarID, entrant.CarID, "A vote is already in progress! Please wait for it to finish.", false)

			if err != nil {
				return err
			}
		}

		/*if vm.currentVote.NumVotes == vm.currentVote.NumConnected {
			vm.StepVote(0xffffffff)
		}*/ //@TODO good idea but would require a lock
	}

	vm.currentVote.VotedIDs[entrant.CarID] = true
	vm.BroadcastVote(entrant.CarID)

	return nil
}

func (vm *VotingManager) StepVote(write uint32) {
	if vm.currentVote == nil {
		return
	}

	votePercent := (float32(vm.currentVote.NumVotes) / float32(vm.currentVote.NumConnected)) * 100

	votingQuorum := float32(vm.state.serverConfig.VotingQuorum)

	if vm.currentVote.VoteType == TCPMessageVoteKick {
		votingQuorum = float32(vm.state.serverConfig.KickQuorum)
	}

	if votePercent < votingQuorum {
		p := NewPacket(nil)

		p.Write(TCPMessageVoteStep)
		p.Write(write) //@TODO wat is this?

		vm.logger.Debugf("Vote failed! Percentage of votes in favour: %.2f, quorum: %.0f", votePercent, votingQuorum)
		vm.state.BroadcastChat(ServerCarID, fmt.Sprintf("Vote failed! Percentage of votes in favour: %.2f, quorum: %.0f", votePercent, votingQuorum), false)

		vm.state.BroadcastAllTCP(p)
	} else {
		// vote passed!
		switch vm.currentVote.VoteType {
		case TCPMessageVoteNextSession:
			vm.logger.Debugf("Vote passed! Moving to next session. Percentage of votes in favour: %.2f, quorum: %.0f", votePercent, votingQuorum)

			vm.sessionManager.NextSession(true, false)
		case TCPMessageVoteRestartSession:
			vm.logger.Debugf("Vote passed! Restarting session. Percentage of votes in favour: %.2f, quorum: %.0f", votePercent, votingQuorum)
			vm.state.BroadcastChat(ServerCarID, fmt.Sprintf("Vote passed! Restarting session. Percentage of votes in favour: %.2f, quorum: %.0f", votePercent, votingQuorum), false)

			vm.sessionManager.RestartSession()
		case TCPMessageVoteKick:
			entrantToKick, err := vm.state.GetCarByID(vm.currentVote.KickID)

			if err == nil {
				vm.logger.Debugf("Vote passed! Kicking %s. Percentage of votes in favour: %.2f, quorum: %.0f", entrantToKick.Driver.Name, votePercent, votingQuorum)
				vm.state.BroadcastChat(ServerCarID, fmt.Sprintf("Vote passed! Kicking %s. Percentage of votes in favour: %.2f, quorum: %.0f", entrantToKick.Driver.Name, votePercent, votingQuorum), false)

				kickReason := KickReasonVotedToBeBanned

				if vm.state.serverConfig.BlockListMode != BlockListModeNormalKick {
					kickReason = KickReasonVotedToBeBlockListed
				}

				if err := vm.state.Kick(entrantToKick.CarID, kickReason); err != nil {
					vm.logger.WithError(err).Errorf("Could not kick car ID: %d", entrantToKick.CarID)
				}
			}
		}
	}

	vm.currentVote = nil
}

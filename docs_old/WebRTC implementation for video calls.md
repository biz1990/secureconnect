# WebRTC Video Call Implementation

## 1. WebRTC Architecture Overview

```
┌──────────────┐                                    ┌──────────────┐
│   Client A   │                                    │   Client B   │
│              │                                    │              │
│ ┌──────────┐ │                                    │ ┌──────────┐ │
│ │  Camera  │ │                                    │ │  Camera  │ │
│ │   Mic    │ │                                    │ │   Mic    │ │
│ └────┬─────┘ │                                    │ └────┬─────┘ │
│      │       │                                    │      │       │
│ ┌────▼─────┐ │      Signaling Messages           │ ┌────▼─────┐ │
│ │ WebRTC   │ │◄──────────────────────────────────┤ │ WebRTC   │ │
│ │  Peer    │ │                                    │ │  Peer    │ │
│ └────┬─────┘ │                                    │ └────┬─────┘ │
│      │       │                                    │      │       │
└──────┼───────┘                                    └──────┼───────┘
       │                                                   │
       │            ┌───────────────────┐                 │
       │            │ Signaling Server  │                 │
       └───────────►│   (WebSocket)     │◄────────────────┘
                    └───────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │   TURN/STUN       │
                    │   Servers         │
                    │   (NAT Traversal) │
                    └───────────────────┘
                              │
       ┌──────────────────────┴──────────────────────┐
       │                                              │
       ▼                                              ▼
┌────────────┐                                  ┌────────────┐
│  Client A  │◄────── P2P Media Stream ────────►│  Client B  │
│  (Audio/   │                                  │  (Audio/   │
│   Video)   │                                  │   Video)   │
└────────────┘                                  └────────────┘
```

## 2. Signaling Server Implementation

### 2.1 WebSocket Signaling Server (Node.js)

```typescript
// signaling-server.ts
import WebSocket from 'ws';
import { createServer } from 'http';
import { authenticate } from './middleware/auth';
import Redis from 'ioredis';

interface CallSession {
  callId: string;
  participants: Map<string, WebSocket>;
  callType: 'audio' | 'video';
  initiator: string;
  createdAt: number;
}

class SignalingServer {
  private wss: WebSocket.Server;
  private activeCalls: Map<string, CallSession>;
  private userConnections: Map<string, WebSocket>;
  private redis: Redis;

  constructor(port: number) {
    const server = createServer();
    this.wss = new WebSocket.Server({ server });
    this.activeCalls = new Map();
    this.userConnections = new Map();
    this.redis = new Redis({
      host: process.env.REDIS_HOST,
      port: parseInt(process.env.REDIS_PORT || '6379')
    });

    this.setupWebSocketServer();
    server.listen(port, () => {
      console.log(`Signaling server running on port ${port}`);
    });
  }

  private setupWebSocketServer(): void {
    this.wss.on('connection', async (ws: WebSocket, req) => {
      console.log('New WebSocket connection');

      // Authenticate connection
      const token = this.extractToken(req.url);
      const user = await authenticate(token);
      
      if (!user) {
        ws.close(1008, 'Authentication failed');
        return;
      }

      const userId = user.id;
      this.userConnections.set(userId, ws);

      // Send connection success
      this.sendMessage(ws, {
        type: 'connected',
        userId
      });

      // Handle messages
      ws.on('message', async (data: string) => {
        try {
          const message = JSON.parse(data);
          await this.handleMessage(userId, ws, message);
        } catch (error) {
          console.error('Message handling error:', error);
          this.sendError(ws, 'Invalid message format');
        }
      });

      // Handle disconnection
      ws.on('close', () => {
        this.handleDisconnect(userId);
      });

      ws.on('error', (error) => {
        console.error('WebSocket error:', error);
      });
    });
  }

  private async handleMessage(userId: string, ws: WebSocket, message: any): Promise<void> {
    switch (message.type) {
      case 'call-initiate':
        await this.handleCallInitiate(userId, message);
        break;

      case 'call-answer':
        await this.handleCallAnswer(userId, message);
        break;

      case 'call-reject':
        await this.handleCallReject(userId, message);
        break;

      case 'call-end':
        await this.handleCallEnd(userId, message);
        break;

      case 'ice-candidate':
        await this.handleIceCandidate(userId, message);
        break;

      case 'offer':
        await this.handleOffer(userId, message);
        break;

      case 'answer':
        await this.handleAnswer(userId, message);
        break;

      case 'renegotiate':
        await this.handleRenegotiate(userId, message);
        break;

      default:
        this.sendError(ws, 'Unknown message type');
    }
  }

  // Call Initiation
  private async handleCallInitiate(initiatorId: string, message: any): Promise<void> {
    const { callId, recipientIds, callType } = message;

    // Create call session
    const callSession: CallSession = {
      callId,
      participants: new Map(),
      callType,
      initiator: initiatorId,
      createdAt: Date.now()
    };

    this.activeCalls.set(callId, callSession);

    // Store in Redis for distributed signaling
    await this.redis.setex(
      `call:${callId}`,
      3600, // 1 hour expiry
      JSON.stringify(callSession)
    );

    // Notify recipients
    for (const recipientId of recipientIds) {
      const recipientWs = this.userConnections.get(recipientId);
      if (recipientWs && recipientWs.readyState === WebSocket.OPEN) {
        this.sendMessage(recipientWs, {
          type: 'incoming-call',
          callId,
          initiatorId,
          callType
        });
      }
    }
  }

  // Handle SDP Offer
  private async handleOffer(senderId: string, message: any): Promise<void> {
    const { callId, recipientId, sdp } = message;

    const recipientWs = this.userConnections.get(recipientId);
    if (!recipientWs || recipientWs.readyState !== WebSocket.OPEN) {
      this.sendError(this.userConnections.get(senderId)!, 'Recipient not available');
      return;
    }

    this.sendMessage(recipientWs, {
      type: 'offer',
      callId,
      senderId,
      sdp
    });
  }

  // Handle SDP Answer
  private async handleAnswer(senderId: string, message: any): Promise<void> {
    const { callId, recipientId, sdp } = message;

    const recipientWs = this.userConnections.get(recipientId);
    if (!recipientWs || recipientWs.readyState !== WebSocket.OPEN) {
      this.sendError(this.userConnections.get(senderId)!, 'Recipient not available');
      return;
    }

    this.sendMessage(recipientWs, {
      type: 'answer',
      callId,
      senderId,
      sdp
    });

    // Update call session status
    const callSession = this.activeCalls.get(callId);
    if (callSession) {
      callSession.participants.set(senderId, this.userConnections.get(senderId)!);
      await this.redis.setex(`call:${callId}`, 3600, JSON.stringify(callSession));
    }
  }

  // Handle ICE Candidates
  private async handleIceCandidate(senderId: string, message: any): Promise<void> {
    const { callId, recipientId, candidate } = message;

    const recipientWs = this.userConnections.get(recipientId);
    if (!recipientWs || recipientWs.readyState !== WebSocket.OPEN) {
      return; // Silently fail for ICE candidates
    }

    this.sendMessage(recipientWs, {
      type: 'ice-candidate',
      callId,
      senderId,
      candidate
    });
  }

  // Handle Call Answer
  private async handleCallAnswer(userId: string, message: any): Promise<void> {
    const { callId } = message;
    const callSession = this.activeCalls.get(callId);

    if (!callSession) {
      this.sendError(this.userConnections.get(userId)!, 'Call not found');
      return;
    }

    // Add participant to call
    callSession.participants.set(userId, this.userConnections.get(userId)!);

    // Notify initiator
    const initiatorWs = this.userConnections.get(callSession.initiator);
    if (initiatorWs && initiatorWs.readyState === WebSocket.OPEN) {
      this.sendMessage(initiatorWs, {
        type: 'call-answered',
        callId,
        userId
      });
    }
  }

  // Handle Call Rejection
  private async handleCallReject(userId: string, message: any): Promise<void> {
    const { callId } = message;
    const callSession = this.activeCalls.get(callId);

    if (!callSession) return;

    // Notify initiator
    const initiatorWs = this.userConnections.get(callSession.initiator);
    if (initiatorWs && initiatorWs.readyState === WebSocket.OPEN) {
      this.sendMessage(initiatorWs, {
        type: 'call-rejected',
        callId,
        userId
      });
    }
  }

  // Handle Call End
  private async handleCallEnd(userId: string, message: any): Promise<void> {
    const { callId } = message;
    const callSession = this.activeCalls.get(callId);

    if (!callSession) return;

    // Notify all participants
    for (const [participantId, participantWs] of callSession.participants) {
      if (participantId !== userId && participantWs.readyState === WebSocket.OPEN) {
        this.sendMessage(participantWs, {
          type: 'call-ended',
          callId,
          endedBy: userId
        });
      }
    }

    // Cleanup
    this.activeCalls.delete(callId);
    await this.redis.del(`call:${callId}`);
  }

  // Handle Renegotiation (add/remove tracks)
  private async handleRenegotiate(senderId: string, message: any): Promise<void> {
    const { callId, recipientId, sdp } = message;

    const recipientWs = this.userConnections.get(recipientId);
    if (!recipientWs || recipientWs.readyState !== WebSocket.OPEN) {
      return;
    }

    this.sendMessage(recipientWs, {
      type: 'renegotiate',
      callId,
      senderId,
      sdp
    });
  }

  // Handle User Disconnect
  private handleDisconnect(userId: string): void {
    console.log(`User ${userId} disconnected`);

    // Find and end all calls for this user
    for (const [callId, callSession] of this.activeCalls) {
      if (callSession.participants.has(userId) || callSession.initiator === userId) {
        this.handleCallEnd(userId, { callId });
      }
    }

    this.userConnections.delete(userId);
  }

  private sendMessage(ws: WebSocket, message: any): void {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message));
    }
  }

  private sendError(ws: WebSocket, error: string): void {
    this.sendMessage(ws, {
      type: 'error',
      error
    });
  }

  private extractToken(url: string | undefined): string | null {
    if (!url) return null;
    const params = new URLSearchParams(url.split('?')[1]);
    return params.get('token');
  }
}

// Start server
const signalingServer = new SignalingServer(8080);

export default SignalingServer;
```

## 3. Client-Side WebRTC Implementation

### 3.1 WebRTC Manager Class

```typescript
// webrtc-manager.ts
class WebRTCManager {
  private peerConnection: RTCPeerConnection | null = null;
  private localStream: MediaStream | null = null;
  private remoteStream: MediaStream | null = null;
  private signalingSocket: WebSocket | null = null;
  private callId: string | null = null;
  private configuration: RTCConfiguration;
  private dataChannel: RTCDataChannel | null = null;

  constructor() {
    this.configuration = {
      iceServers: [
        {
          urls: 'stun:stun.l.google.com:19302'
        },
        {
          urls: 'stun:stun1.l.google.com:19302'
        },
        {
          urls: 'turn:your-turn-server.com:3478',
          username: 'username',
          credential: 'password'
        }
      ],
      iceTransportPolicy: 'all',
      bundlePolicy: 'max-bundle',
      rtcpMuxPolicy: 'require',
      iceCandidatePoolSize: 10
    };
  }

  // Connect to signaling server
  async connectSignaling(token: string): Promise<void> {
    return new Promise((resolve, reject) => {
      this.signalingSocket = new WebSocket(`wss://signal.yourdomain.com?token=${token}`);

      this.signalingSocket.onopen = () => {
        console.log('Connected to signaling server');
        resolve();
      };

      this.signalingSocket.onerror = (error) => {
        console.error('Signaling connection error:', error);
        reject(error);
      };

      this.signalingSocket.onmessage = async (event) => {
        const message = JSON.parse(event.data);
        await this.handleSignalingMessage(message);
      };

      this.signalingSocket.onclose = () => {
        console.log('Signaling connection closed');
      };
    });
  }

  // Get local media stream
  async getLocalStream(constraints: MediaStreamConstraints): Promise<MediaStream> {
    try {
      const stream = await navigator.mediaDevices.getUserMedia(constraints);
      this.localStream = stream;
      return stream;
    } catch (error) {
      console.error('Error accessing media devices:', error);
      throw error;
    }
  }

  // Get screen share stream
  async getScreenShareStream(): Promise<MediaStream> {
    try {
      const stream = await navigator.mediaDevices.getDisplayMedia({
        video: {
          cursor: 'always',
          displaySurface: 'monitor'
        },
        audio: false
      });
      return stream;
    } catch (error) {
      console.error('Error accessing screen share:', error);
      throw error;
    }
  }

  // Initiate call
  async initiateCall(recipientId: string, callType: 'audio' | 'video'): Promise<string> {
    this.callId = `call_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

    // Get local media
    const constraints: MediaStreamConstraints = {
      audio: {
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
        sampleRate: 48000
      },
      video: callType === 'video' ? {
        width: { ideal: 1280 },
        height: { ideal: 720 },
        frameRate: { ideal: 30 },
        facingMode: 'user'
      } : false
    };

    this.localStream = await this.getLocalStream(constraints);

    // Create peer connection
    this.createPeerConnection(recipientId);

    // Add local tracks
    this.localStream.getTracks().forEach(track => {
      this.peerConnection!.addTrack(track, this.localStream!);
    });

    // Create data channel
    this.dataChannel = this.peerConnection!.createDataChannel('chat', {
      ordered: true
    });
    this.setupDataChannel(this.dataChannel);

    // Create and send offer
    const offer = await this.peerConnection!.createOffer({
      offerToReceiveAudio: true,
      offerToReceiveVideo: callType === 'video'
    });

    await this.peerConnection!.setLocalDescription(offer);

    // Send offer via signaling
    this.sendSignalingMessage({
      type: 'offer',
      callId: this.callId,
      recipientId,
      sdp: offer.sdp
    });

    // Notify server about call initiation
    this.sendSignalingMessage({
      type: 'call-initiate',
      callId: this.callId,
      recipientIds: [recipientId],
      callType
    });

    return this.callId;
  }

  // Answer incoming call
  async answerCall(callId: string, senderId: string, offer: RTCSessionDescriptionInit): Promise<void> {
    this.callId = callId;

    // Get local media
    const constraints: MediaStreamConstraints = {
      audio: {
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true
      },
      video: {
        width: { ideal: 1280 },
        height: { ideal: 720 },
        frameRate: { ideal: 30 }
      }
    };

    this.localStream = await this.getLocalStream(constraints);

    // Create peer connection
    this.createPeerConnection(senderId);

    // Add local tracks
    this.localStream.getTracks().forEach(track => {
      this.peerConnection!.addTrack(track, this.localStream!);
    });

    // Set remote description
    await this.peerConnection!.setRemoteDescription(new RTCSessionDescription(offer));

    // Create answer
    const answer = await this.peerConnection!.createAnswer();
    await this.peerConnection!.setLocalDescription(answer);

    // Send answer
    this.sendSignalingMessage({
      type: 'answer',
      callId: this.callId,
      recipientId: senderId,
      sdp: answer.sdp
    });

    // Notify server
    this.sendSignalingMessage({
      type: 'call-answer',
      callId: this.callId
    });
  }

  // Create peer connection
  private createPeerConnection(remoteUserId: string): void {
    this.peerConnection = new RTCPeerConnection(this.configuration);

    // Handle ICE candidates
    this.peerConnection.onicecandidate = (event) => {
      if (event.candidate) {
        this.sendSignalingMessage({
          type: 'ice-candidate',
          callId: this.callId,
          recipientId: remoteUserId,
          candidate: event.candidate.toJSON()
        });
      }
    };

    // Handle ICE connection state changes
    this.peerConnection.oniceconnectionstatechange = () => {
      console.log('ICE connection state:', this.peerConnection?.iceConnectionState);
      
      if (this.peerConnection?.iceConnectionState === 'failed') {
        // Attempt ICE restart
        this.restartIce(remoteUserId);
      }
    };

    // Handle connection state changes
    this.peerConnection.onconnectionstatechange = () => {
      console.log('Connection state:', this.peerConnection?.connectionState);
      
      if (this.peerConnection?.connectionState === 'connected') {
        this.onCallConnected?.();
      } else if (this.peerConnection?.connectionState === 'disconnected') {
        this.onCallDisconnected?.();
      } else if (this.peerConnection?.connectionState === 'failed') {
        this.onCallFailed?.();
      }
    };

    // Handle remote tracks
    this.peerConnection.ontrack = (event) => {
      console.log('Remote track received:', event.track.kind);
      
      if (!this.remoteStream) {
        this.remoteStream = new MediaStream();
        this.onRemoteStream?.(this.remoteStream);
      }
      
      this.remoteStream.addTrack(event.track);
    };

    // Handle data channel
    this.peerConnection.ondatachannel = (event) => {
      this.dataChannel = event.channel;
      this.setupDataChannel(this.dataChannel);
    };

    // Handle negotiation needed
    this.peerConnection.onnegotiationneeded = async () => {
      console.log('Negotiation needed');
      try {
        const offer = await this.peerConnection!.createOffer();
        await this.peerConnection!.setLocalDescription(offer);
        
        this.sendSignalingMessage({
          type: 'renegotiate',
          callId: this.callId,
          recipientId: remoteUserId,
          sdp: offer.sdp
        });
      } catch (error) {
        console.error('Renegotiation error:', error);
      }
    };
  }

  // Handle signaling messages
  private async handleSignalingMessage(message: any): Promise<void> {
    switch (message.type) {
      case 'connected':
        console.log('Connected to signaling server, userId:', message.userId);
        break;

      case 'incoming-call':
        this.onIncomingCall?.(message);
        break;

      case 'offer':
        await this.handleOffer(message);
        break;

      case 'answer':
        await this.handleAnswer(message);
        break;

      case 'ice-candidate':
        await this.handleIceCandidate(message);
        break;

      case 'call-answered':
        this.onCallAnswered?.(message);
        break;

      case 'call-rejected':
        this.onCallRejected?.(message);
        break;

      case 'call-ended':
        this.handleCallEnded(message);
        break;

      case 'renegotiate':
        await this.handleRenegotiate(message);
        break;

      case 'error':
        console.error('Signaling error:', message.error);
        this.onError?.(message.error);
        break;
    }
  }

  private async handleOffer(message: any): Promise<void> {
    // This is handled in answerCall()
  }

  private async handleAnswer(message: any): Promise<void> {
    const answer = new RTCSessionDescription({
      type: 'answer',
      sdp: message.sdp
    });

    await this.peerConnection?.setRemoteDescription(answer);
  }

  private async handleIceCandidate(message: any): Promise<void> {
    const candidate = new RTCIceCandidate(message.candidate);
    await this.peerConnection?.addIceCandidate(candidate);
  }

  private async handleRenegotiate(message: any): Promise<void> {
    const offer = new RTCSessionDescription({
      type: 'offer',
      sdp: message.sdp
    });

    await this.peerConnection?.setRemoteDescription(offer);

    const answer = await this.peerConnection?.createAnswer();
    await this.peerConnection?.setLocalDescription(answer!);

    this.sendSignalingMessage({
      type: 'answer',
      callId: message.callId,
      recipientId: message.senderId,
      sdp: answer?.sdp
    });
  }

  // Toggle audio
  toggleAudio(enabled: boolean): void {
    if (this.localStream) {
      this.localStream.getAudioTracks().forEach(track => {
        track.enabled = enabled;
      });
    }
  }

  // Toggle video
  toggleVideo(enabled: boolean): void {
    if (this.localStream) {
      this.localStream.getVideoTracks().forEach(track => {
        track.enabled = enabled;
      });
    }
  }

  // Switch camera (front/back)
  async switchCamera(): Promise<void> {
    if (!this.localStream) return;

    const videoTrack = this.localStream.getVideoTracks()[0];
    const constraints = videoTrack.getConstraints();
    
    // @ts-ignore
    const currentFacingMode = constraints.facingMode;
    const newFacingMode = currentFacingMode === 'user' ? 'environment' : 'user';

    // Stop current track
    videoTrack.stop();

    // Get new stream with switched camera
    const newStream = await navigator.mediaDevices.getUserMedia({
      video: { facingMode: newFacingMode }
    });

    const newVideoTrack = newStream.getVideoTracks()[0];

    // Replace track in peer connection
    const sender = this.peerConnection?.getSenders().find(s => s.track?.kind === 'video');
    if (sender) {
      await sender.replaceTrack(newVideoTrack);
    }

    // Update local stream
    this.localStream.removeTrack(videoTrack);
    this.localStream.addTrack(newVideoTrack);
  }

  // Share screen
  async shareScreen(): Promise<void> {
    const screenStream = await this.getScreenShareStream();
    const screenTrack = screenStream.getVideoTracks()[0];

    // Replace video track with screen track
    const sender = this.peerConnection?.getSenders().find(s => s.track?.kind === 'video');
    if (sender) {
      await sender.replaceTrack(screenTrack);
    }

    // Handle screen share stop
    screenTrack.onended = async () => {
      // Switch back to camera
      if (this.localStream) {
        const cameraTrack = this.localStream.getVideoTracks()[0];
        if (sender) {
          await sender.replaceTrack(cameraTrack);
        }
      }
    };
  }

  // End call
  async endCall(): Promise<void> {
    // Send end call signal
    if (this.callId) {
      this.sendSignalingMessage({
        type: 'call-end',
        callId: this.callId
      });
    }

    this.cleanup();
  }

  private handleCallEnded(message: any): void {
    this.onCallEnded?.(message);
    this.cleanup();
  }

  private cleanup(): void {
    // Stop local tracks
    this.localStream?.getTracks().forEach(track => track.stop());

    // Close peer connection
    this.peerConnection?.close();

    // Close data channel
    this.dataChannel?.close();

    // Clear references
    this.localStream = null;
    this.remoteStream = null;
    this.peerConnection = null;
    this.dataChannel = null;
    this.callId = null;
  }

  // ICE restart
  private async restartIce(remoteUserId: string): Promise<void> {
    if (!this.peerConnection) return;

    const offer = await this.peerConnection.createOffer({ iceRestart: true });
    await this.peerConnection.setLocalDescription(offer);

    this.sendSignalingMessage({
      type: 'offer',
      callId: this.callId,
      recipientId: remoteUserId,
      sdp: offer.sdp
    });
  }

  // Data channel setup
  private setupDataChannel(channel: RTCDataChannel): void {
    channel.onopen = () => {
      console.log('Data channel opened');
      this.onDataChannelOpen?.();
    };

    channel.onclose = () => {
      console.log('Data channel closed');
    };

    channel.onmessage = (event) => {
      this.onDataChannelMessage?.(event.data);
    };

    channel.onerror = (error) => {
      console.error('Data channel error:', error);
    };
  }

  // Send data via data channel
  sendData(data: string): void {
    if (this.dataChannel && this.dataChannel.readyState === 'open') {
      this.dataChannel.send(data);
    }
  }

  // Send signaling message
  private sendSignalingMessage(message: any): void {
    if (this.signalingSocket && this.signalingSocket.readyState === WebSocket.OPEN) {
      this.signalingSocket.send(JSON.stringify(message));
    }
  }

  // Get call statistics
  async getStats(): Promise<RTCStatsReport | null> {
    if (!this.peerConnection) return null;
    return await this.peerConnection.getStats();
  }

  // Event handlers (to be set by UI)
  onIncomingCall?: (data: any) => void;
  onCallAnswered?: (data: any) => void;
  onCallRejected?: (data: any) => void;
  onCallEnded?: (data: any) => void;
  onCallConnected?: () => void;
  onCallDisconnected?: () => void;
  onCallFailed?: () => void;
  onRemoteStream?: (stream: MediaStream) => void;
  onDataChannelOpen?: () => void;
  onDataChannelMessage?: (data: string) => void;
  onError?: (error: string) => void;
}

export default WebRTCManager;
```

## 4. React Component Example

```typescript
// VideoCall.tsx
import React, { useEffect, useRef, useState } from 'react';
import WebRTCManager from './webrtc-manager';

interface VideoCallProps {
  recipientId: string;
  callType: 'audio' | 'video';
  token: string;
}

const VideoCall: React.Component<VideoCallProps> = ({ recipientId, callType, token }) => {
  const localVideoRef = useRef<HTMLVideoElement>(null);
  const remoteVideoRef = useRef<HTMLVideoElement>(null);
  const [webrtc] = useState(() => new WebRTCManager());
  const [isAudioEnabled, setIsAudioEnabled] = useState(true);
  const [isVideoEnabled, setIsVideoEnabled] = useState(true);
  const [isCallActive, setIsCallActive] = useState(false);
  const [callStats, setCallStats] = useState<any>(null);

  useEffect(() => {
    initializeCall();

    return () => {
      webrtc.endCall();
    };
  }, []);

  const initializeCall = async () => {
    try {
      // Connect to signaling server
      await webrtc.connectSignaling(token);

      // Setup event handlers
      webrtc.onRemoteStream = (stream) => {
        if (remoteVideoRef.current) {
          remoteVideoRef.current.srcObject = stream;
        }
      };

      webrtc.onCallConnected = () => {
        setIsCallActive(true);
        console.log('Call connected');
      };

      webrtc.onCallEnded = () => {
        setIsCallActive(false);
        console.log('Call ended');
      };

      // Initiate call
      const callId = await webrtc.initiateCall(recipientId, callType);

      // Display local stream
      if (localVideoRef.current && webrtc.localStream) {
        localVideoRef.current.srcObject = webrtc.localStream;
      }

      // Monitor call stats
      monitorCallStats();
    } catch (error) {
      console.error('Call initialization error:', error);
    }
  };

  const monitorCallStats = () => {
    const interval = setInterval(async () => {
      const stats = await webrtc.getStats();
      if (stats) {
        setCallStats(parseStats(stats));
      }
    }, 1000);

    return () => clearInterval(interval);
  };

  const parseStats = (stats: RTCStatsReport) => {
    let audioLevel = 0;
    let videoResolution = '';
    let bitrate = 0;

    stats.forEach((report) => {
      if (report.type === 'inbound-rtp' && report.kind === 'video') {
        videoResolution = `${report.frameWidth}x${report.frameHeight}`;
        bitrate = report.bytesReceived * 8 / 1000; // kbps
      }
      if (report.type === 'media-source' && report.kind === 'audio') {
        audioLevel = report.audioLevel;
      }
    });

    return { audioLevel, videoResolution, bitrate };
  };

  const toggleAudio = () => {
    const newState = !isAudioEnabled;
    webrtc.toggleAudio(newState);
    setIsAudioEnabled(newState);
  };

  const toggleVideo = () => {
    const newState = !isVideoEnabled;
    webrtc.toggleVideo(newState);
    setIsVideoEnabled(newState);
  };

  const handleShareScreen = async () => {
    try {
      await webrtc.shareScreen();
    } catch (error) {
      console.error('Screen share error:', error);
    }
  };

  const handleSwitchCamera = async () => {
    try {
      await webrtc.switchCamera();
    } catch (error) {
      console.error('Camera switch error:', error);
    }
  };

  const handleEndCall = async () => {
    await webrtc.endCall();
    setIsCallActive(false);
  };

  return (
    <div className="video-call-container">
      <div className="video-grid">
        {/* Remote video */}
        <video
          ref={remoteVideoRef}
          autoPlay
          playsInline
          className="remote-video"
        />

        {/* Local video */}
        <video
          ref={localVideoRef}
          autoPlay
          playsInline
          muted
          className="local-video"
        />
      </div>

      {/* Call stats */}
      {callStats && (
        <div className="call-stats">
          <p>Resolution: {callStats.videoResolution}</p>
          <p>Bitrate: {callStats.bitrate} kbps</p>
        </div>
      )}

      {/* Call controls */}
      <div className="call-controls">
        <button onClick={toggleAudio} className={isAudioEnabled ? 'active' : ''}>
          {isAudioEnabled ? '🎤' : '🔇'}
        </button>

        <button onClick={toggleVideo} className={isVideoEnabled ? 'active' : ''}>
          {isVideoEnabled ? '📹' : '📷'}
        </button>

        <button onClick={handleShareScreen}>
          🖥️ Share Screen
        </button>

        <button onClick={handleSwitchCamera}>
          🔄 Switch Camera
        </button>

        <button onClick={handleEndCall} className="end-call">
          ❌ End Call
        </button>
      </div>
    </div>
  );
};

export default VideoCall;
```

## 5. Group Video Call (SFU Architecture)

### 5.1 Mediasoup SFU Server

```typescript
// mediasoup-server.ts
import * as mediasoup from 'mediasoup';
import { Router, Transport, Producer, Consumer, Worker } from 'mediasoup/node/lib/types';

interface Room {
  id: string;
  router: Router;
  peers: Map<string, Peer>;
}

interface Peer {
  id: string;
  transports: Map<string, Transport>;
  producers: Map<string, Producer>;
  consumers: Map<string, Consumer>;
}

class MediasoupServer {
  private workers: Worker[] = [];
  private rooms: Map<string, Room> = new Map();
  private nextWorkerIndex = 0;

  async initialize(): Promise<void> {
    // Create workers
    const numWorkers = 4;
    for (let i = 0; i < numWorkers; i++) {
      const worker = await mediasoup.createWorker({
        logLevel: 'warn',
        rtcMinPort: 10000 + (i * 1000),
        rtcMaxPort: 10000 + (i * 1000) + 999
      });

      worker.on('died', () => {
        console.error('Worker died, exiting...');
        process.exit(1);
      });

      this.workers.push(worker);
    }

    console.log(`Created ${numWorkers} mediasoup workers`);
  }

  // Create room
  async createRoom(roomId: string): Promise<Room> {
    // Get next worker
    const worker = this.workers[this.nextWorkerIndex];
    this.nextWorkerIndex = (this.nextWorkerIndex + 1) % this.workers.length;

    // Create router
    const router = await worker.createRouter({
      mediaCodecs: [
        {
          kind: 'audio',
          mimeType: 'audio/opus',
          clockRate: 48000,
          channels: 2
        },
        {
          kind: 'video',
          mimeType: 'video/VP8',
          clockRate: 90000,
          parameters: {
            'x-google-start-bitrate': 1000
          }
        },
        {
          kind: 'video',
          mimeType: 'video/H264',
          clockRate: 90000,
          parameters: {
            'packetization-mode': 1,
            'profile-level-id': '42e01f',
            'level-asymmetry-allowed': 1
          }
        }
      ]
    });

    const room: Room = {
      id: roomId,
      router,
      peers: new Map()
    };

    this.rooms.set(roomId, room);
    return room;
  }

  // Join room
  async joinRoom(roomId: string, peerId: string): Promise<{
    rtpCapabilities: any
  }> {
    let room = this.rooms.get(roomId);
    if (!room) {
      room = await this.createRoom(roomId);
    }

    const peer: Peer = {
      id: peerId,
      transports: new Map(),
      producers: new Map(),
      consumers: new Map()
    };

    room.peers.set(peerId, peer);

    return {
      rtpCapabilities: room.router.rtpCapabilities
    };
  }

  // Create WebRTC transport
  async createTransport(roomId: string, peerId: string): Promise<{
    id: string;
    iceParameters: any;
    iceCandidates: any;
    dtlsParameters: any;
  }> {
    const room = this.rooms.get(roomId);
    if (!room) throw new Error('Room not found');

    const peer = room.peers.get(peerId);
    if (!peer) throw new Error('Peer not found');

    const transport = await room.router.createWebRtcTransport({
      listenIps: [
        {
          ip: '0.0.0.0',
          announcedIp: 'YOUR_PUBLIC_IP' // Replace with your server's public IP
        }
      ],
      enableUdp: true,
      enableTcp: true,
      preferUdp: true
    });

    peer.transports.set(transport.id, transport);

    return {
      id: transport.id,
      iceParameters: transport.iceParameters,
      iceCandidates: transport.iceCandidates,
      dtlsParameters: transport.dtlsParameters
    };
  }

  // Connect transport
  async connectTransport(
    roomId: string,
    peerId: string,
    transportId: string,
    dtlsParameters: any
  ): Promise<void> {
    const room = this.rooms.get(roomId);
    if (!room) throw new Error('Room not found');

    const peer = room.peers.get(peerId);
    if (!peer) throw new Error('Peer not found');

    const transport = peer.transports.get(transportId);
    if (!transport) throw new Error('Transport not found');

    await transport.connect({ dtlsParameters });
  }

  // Produce media
  async produce(
    roomId: string,
    peerId: string,
    transportId: string,
    kind: 'audio' | 'video',
    rtpParameters: any
  ): Promise<string> {
    const room = this.rooms.get(roomId);
    if (!room) throw new Error('Room not found');

    const peer = room.peers.get(peerId);
    if (!peer) throw new Error('Peer not found');

    const transport = peer.transports.get(transportId);
    if (!transport) throw new Error('Transport not found');

    const producer = await transport.produce({
      kind,
      rtpParameters
    });

    peer.producers.set(producer.id, producer);

    // Notify other peers
    this.notifyNewProducer(room, peerId, producer.id, kind);

    return producer.id;
  }

  // Consume media
  async consume(
    roomId: string,
    peerId: string,
    transportId: string,
    producerId: string,
    rtpCapabilities: any
  ): Promise<{
    id: string;
    kind: string;
    rtpParameters: any;
  }> {
    const room = this.rooms.get(roomId);
    if (!room) throw new Error('Room not found');

    const peer = room.peers.get(peerId);
    if (!peer) throw new Error('Peer not found');

    const transport = peer.transports.get(transportId);
    if (!transport) throw new Error('Transport not found');

    // Check if can consume
    if (!room.router.canConsume({ producerId, rtpCapabilities })) {
      throw new Error('Cannot consume');
    }

    const consumer = await transport.consume({
      producerId,
      rtpCapabilities,
      paused: false
    });

    peer.consumers.set(consumer.id, consumer);

    return {
      id: consumer.id,
      kind: consumer.kind,
      rtpParameters: consumer.rtpParameters
    };
  }

  // Resume consumer
  async resumeConsumer(roomId: string, peerId: string, consumerId: string): Promise<void> {
    const room = this.rooms.get(roomId);
    if (!room) throw new Error('Room not found');

    const peer = room.peers.get(peerId);
    if (!peer) throw new Error('Peer not found');

    const consumer = peer.consumers.get(consumerId);
    if (!consumer) throw new Error('Consumer not found');

    await consumer.resume();
  }

  // Leave room
  async leaveRoom(roomId: string, peerId: string): Promise<void> {
    const room = this.rooms.get(roomId);
    if (!room) return;

    const peer = room.peers.get(peerId);
    if (!peer) return;

    // Close all transports
    peer.transports.forEach(transport => transport.close());

    // Remove peer
    room.peers.delete(peerId);

    // Delete room if empty
    if (room.peers.size === 0) {
      room.router.close();
      this.rooms.delete(roomId);
    }
  }

  private notifyNewProducer(room: Room, producerPeerId: string, producerId: string, kind: string): void {
    // Notify all other peers about new producer
    room.peers.forEach((peer, peerId) => {
      if (peerId !== producerPeerId) {
        // Send notification via WebSocket
        // This would be handled by your signaling server
        console.log(`Notify ${peerId} about new producer ${producerId}`);
      }
    });
  }
}

export default new MediasoupServer();
```

## 6. TURN Server Configuration (Coturn)

```bash
# /etc/turnserver.conf
listening-port=3478
tls-listening-port=5349

# External IP
external-ip=YOUR_PUBLIC_IP

# Relay IP
relay-ip=YOUR_PUBLIC_IP

# Port range for relay
min-port=49152
max-port=65535

# Authentication
lt-cred-mech
user=username:password

# Realm
realm=yourdomain.com

# SSL certificates
cert=/etc/letsencrypt/live/yourdomain.com/cert.pem
pkey=/etc/letsencrypt/live/yourdomain.com/privkey.pem

# Logging
log-file=/var/log/turnserver.log
verbose

# Security
no-tlsv1
no-tlsv1_1
no-stdout-log
```

## 7. Call Quality Monitoring

```typescript
// call-quality-monitor.ts
class CallQualityMonitor {
  private peerConnection: RTCPeerConnection;
  private intervalId: NodeJS.Timeout | null = null;
  private metrics: CallMetrics = {
    audioPacketsLost: 0,
    videoPacketsLost: 0,
    audioJitter: 0,
    videoJitter: 0,
    roundTripTime: 0,
    availableBandwidth: 0
  };

  constructor(peerConnection: RTCPeerConnection) {
    this.peerConnection = peerConnection;
  }

  start(callback: (metrics: CallMetrics) => void): void {
    this.intervalId = setInterval(async () => {
      const stats = await this.peerConnection.getStats();
      this.metrics = this.parseStats(stats);
      callback(this.metrics);
    }, 1000);
  }

  stop(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  private parseStats(stats: RTCStatsReport): CallMetrics {
    const metrics: CallMetrics = {
      audioPacketsLost: 0,
      videoPacketsLost: 0,
      audioJitter: 0,
      videoJitter: 0,
      roundTripTime: 0,
      availableBandwidth: 0
    };

    stats.forEach((report) => {
      if (report.type === 'inbound-rtp') {
        if (report.kind === 'audio') {
          metrics.audioPacketsLost = report.packetsLost || 0;
          metrics.audioJitter = report.jitter || 0;
        } else if (report.kind === 'video') {
          metrics.videoPacketsLost = report.packetsLost || 0;
          metrics.videoJitter = report.jitter || 0;
        }
      }

      if (report.type === 'candidate-pair' && report.state === 'succeeded') {
        metrics.roundTripTime = report.currentRoundTripTime || 0;
      }

      if (report.type === 'transport') {
        metrics.availableBandwidth = report.availableOutgoingBitrate || 0;
      }
    });

    return metrics;
  }

  getQualityRating(): 'excellent' | 'good' | 'fair' | 'poor' {
    const { audioPacketsLost, videoPacketsLost, roundTripTime } = this.metrics;
    
    const totalPacketsLost = audioPacketsLost + videoPacketsLost;
    
    if (totalPacketsLost < 10 && roundTripTime < 0.1) {
      return 'excellent';
    } else if (totalPacketsLost < 50 && roundTripTime < 0.2) {
      return 'good';
    } else if (totalPacketsLost < 100 && roundTripTime < 0.3) {
      return 'fair';
    } else {
      return 'poor';
    }
  }
}

interface CallMetrics {
  audioPacketsLost: number;
  videoPacketsLost: number;
  audioJitter: number;
  videoJitter: number;
  roundTripTime: number;
  availableBandwidth: number;
}

export default CallQualityMonitor;
```

## 8. Deployment Considerations

### 8.1 Infrastructure Requirements

```yaml
# Minimum infrastructure for production

Signaling Servers:
  - 2+ instances (load balanced)
  - WebSocket support
  - Redis for state management

STUN Servers:
  - Use Google's free STUN servers
  - Or deploy your own

TURN Servers:
  - 2+ instances per region
  - High bandwidth required
  - Load balanced

SFU Servers (for group calls):
  - 4+ instances per region
  - Mediasoup/Janus
  - Auto-scaling based on active calls

Database:
  - PostgreSQL for call logs
  - Redis for real-time state
```

### 8.2 Bandwidth Requirements

```
Per User (Video Call):
- 720p: 1-2 Mbps
- 1080p: 2-4 Mbps
- 4K: 8-16 Mbps

Per User (Audio Call):
- Standard: 40-50 Kbps
- HD Audio: 128 Kbps

Group Call (per participant):
- Receives: N-1 streams
- Sends: 1 stream
- SFU reduces bandwidth significantly
```

## 9. Security Best Practices

```
1. DTLS-SRTP encryption (mandatory)
2. Token-based authentication
3. Rate limiting on signaling
4. IP whitelisting for TURN servers
5. Encrypted signaling (WSS)
6. Certificate validation
7. No unencrypted media streams
8. Regular security audits
```

## 10. Testing Checklist

```
✓ 1-1 audio call
✓ 1-1 video call
✓ Group audio call (3-10 participants)
✓ Group video call (3-10 participants)
✓ Screen sharing
✓ Camera switching
✓ Audio/video toggle
✓ Network quality adaptation
✓ Call reconnection on network loss
✓ Cross-browser compatibility
✓ Mobile responsiveness
✓ Call recording
✓ Call statistics
```
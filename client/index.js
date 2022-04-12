//Dragon's Eye © Albert Bregonia 2021

// set up connections

function formatSignal(event, data) {
    return JSON.stringify({ 
        event: event, 
        data: JSON.stringify(data)
    });
}

const ws = new WebSocket(`wss://${location.hostname}:${location.port}/signaler`); //create a websocket for WebRTC signaling 
let rtc, chan;
ws.onopen = () => {
    console.log(`Connected`);
    rtc = new RTCPeerConnection({iceServers: [{urls: `stun:stun.l.google.com:19302`}]}); //create a WebRTC instance
    rtc.onicecandidate = ({candidate}) => candidate && ws.send(formatSignal(`ice`, candidate)); //if the ice candidate is not null, send it to the peer
    rtc.oniceconnectionstatechange = () => rtc.iceConnectionState == `failed` && rtc.restartIce();
    rtc.onconnectionstatechange = (e) => console.log(e);
    chan = rtc.createDataChannel(`sample`);
    chan.onmessage = ({data}) => console.log(data);
    ws.onmessage = async ({data}) => { //signal handler
        const signal = JSON.parse(data),
            content = JSON.parse(signal.data);
        switch(signal.event) {
            case `offer`:
                console.log(`got offer!`, content);
                await rtc.setRemoteDescription(content); //accept offer
                const answer = await rtc.createAnswer();
                await rtc.setLocalDescription(answer);
                ws.send(formatSignal(`answer`, answer)); //send answer
                console.log(`sent answer!`, answer);
                break;
            case `answer`:
                console.log(`got answer!`, content);
                await rtc.setRemoteDescription(content); //accept answer
                break;
            case `ice`:
                console.log(`got ice!`, content);
                rtc.addIceCandidate(content); //add ice candidates
                break;
            case `error`:
                alert();
                break;
            default:
                console.log(`Invalid message:`, content);
        }
    };
    (async () => {
        const offer = await rtc.createOffer();
        await rtc.setLocalDescription(offer);
        ws.send(formatSignal(`offer`, offer)); //send offer
        console.log(`sent offer!`, offer);
    })();
};
ws.onclose = ws.onerror = ({reason}) => alert(`Disconnected ${reason}`);
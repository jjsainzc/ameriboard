export interface YsfStation {
  callsign: string;
  ip: string;
  gateway: string;
  since: string;
}

export interface YsfActivity {
  time: string;
  callsign: string;
  target: string;
  gateway: string;
  duration: string;
  loss: string;
}

export interface DmrActivity {
  time: string;
  callsign: string;
  dmrId: string;
  talkgroup: string;
  source: string;
  duration: string;
}

export interface DVSwitchActivity {
  rxTime: string;
  mode: string;
  callsign: string;
  name: string;
  target: string;
  source: string;
  duration: string;
  loss: string;
  ber: string;
}

export interface EchoLinkStation {
  number: string;
  user: string;
  attribute: string;
}

export interface EchoLinkLastHeard {
  callsign: string;
  disconnectedAt: string;
  duration: string;
}

export interface DVSwitchBridge {
  modeA: string;
  modeB: string;
}

export interface YsfReflectorInfo {
  available: boolean;
  id: string;
  name: string;
  description: string;
  port: string;
}

export interface PysfReflectorData {
  available: boolean;
  id: string;
  name: string;
  description: string;
  port: string;
  connected: YsfStation[];
  lastHeard: YsfActivity[];
}

export interface ReflectorDashboardData {
  ysfAvailable: boolean;
  ysfTrafficAvailable: boolean;
  ysfReflectorInfo: YsfReflectorInfo;
  pysfReflector: PysfReflectorData;
  nxdnReflector: {
    available: boolean;
    tg: string;
    port: string;
    connected: YsfStation[];
    lastHeard: YsfActivity[];
  };
  dmrAvailable: boolean;
  echolinkAvailable: boolean;
  warnings: string[];
  header: {
    title: string;
    subtitle: string;
    badge: string;
    localBridgeStatus: string;
    lastUpdate: string;
  };
  dvSwitchTitle: string;
  nxdnTitle: string;
  echolinkTitle: string;
  dvSwitchBridges: DVSwitchBridge[];
  stats: {
    ysfConnectedCount: number;
    ysfTx24h: number;
    dmrTx24h: number;
    dvSwitchTx24h: number;
    stations24h: number;
  };
  ysfConnected: YsfStation[];
  ysfLastHeard: YsfActivity[];
  dmrLastHeard: DmrActivity[];
  dvSwitchLastHeard: DVSwitchActivity[];
  echolinkConnected: EchoLinkStation[];
  echolinkLastHeard: EchoLinkLastHeard[];
  echolinkUptime: string;
}

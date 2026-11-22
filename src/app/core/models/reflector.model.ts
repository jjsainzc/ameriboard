export interface YsfStation {
  callsign: string;
  gateway: string;
  duration: string;
  status: 'TX' | 'IDLE';
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

export interface ReflectorDashboardData {
  header: {
    title: string;
    subtitle: string;
    badge: string;
    localBridgeStatus: string;
    lastUpdate: string;
  };
  stats: {
    ysfConnectedCount: number;
    ysfTotalHeardToday: number;
    dmrActiveCount: number;
    dmrTotalHeardToday: number;
  };
  ysfConnected: YsfStation[];
  ysfLastHeard: YsfActivity[];
  dmrLastHeard: DmrActivity[];
}

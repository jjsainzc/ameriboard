import { DOCUMENT } from '@angular/common';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { ReflectorService } from './reflector.service';

describe('ReflectorService', () => {
  let base: HTMLBaseElement;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    const document = TestBed.inject(DOCUMENT);
    base = document.createElement('base');
    base.setAttribute('href', '/ameriboard/');
    document.head.prepend(base);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    base.remove();
    http.verify();
  });

  it('requests the configured base path and maps the backend response', () => {
    const service = TestBed.inject(ReflectorService);
    service.fetchStatusOnce().subscribe();
    http.expectOne('/ameriboard/api/reflector-status').flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 1, ysfTx24h: 2, dmrTx24h: 3, stations24h: 4 },
      ysfConnectedList: [{ callsign: 'TEST1', ip: '192.0.2.1', type: 'Gateway', since: '10:00' }],
      ysfLastTx: [{ callsign: 'TEST1', duration: '5s', ago: 'hace 1m' }],
      dmrLastTx: null
    });
    expect(service.data()?.stats).toEqual({ ysfConnectedCount: 1, ysfTx24h: 2, dmrTx24h: 3, stations24h: 4 });
    expect(service.data()?.ysfConnected[0]).toEqual({ callsign: 'TEST1', ip: '192.0.2.1', gateway: 'Gateway', since: '10:00' });
    expect(service.data()?.ysfLastHeard[0].loss).toBe('—');
    expect(service.data()?.dmrLastHeard).toEqual([]);
    expect(service.error()).toBeNull();
  });

  it('reports an invalid response instead of showing success', () => {
    const service = TestBed.inject(ReflectorService);
    service.fetchStatusOnce().subscribe();
    http.expectOne('/ameriboard/api/reflector-status').flush({ status: 'ok' });
    expect(service.error()).toBeTruthy();
    expect(service.data()).toBeNull();
    expect(service.loading()).toBe(false);
  });
});

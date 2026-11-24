import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { parseBannedStations, ReflectorDashboardComponent } from './reflector-dashboard.component';
import { ReflectorService } from '../../core/services/reflector.service';

describe('ReflectorDashboardComponent', () => {
  it('parses theLinkBox ban lists without treating errors as an empty list', () => {
    expect(parseBannedStations('tlbcmd Version 0.56\n0\nbanned users:\nW1AW -\nKQ4ABC Nombre 192.0.2.1\n'))
      .toEqual(['W1AW', 'KQ4ABC']);
    expect(parseBannedStations('0\nThe banned list is empty.\n')).toEqual([]);
    expect(() => parseBannedStations('0\nunexpected response')).toThrow();
    expect(() => parseBannedStations('0\nbanned users:\nW1AW;shutdown -')).toThrow();
  });

  it('paginates independently only above 25 rows and handles filtering and live updates', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 25, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 },
      ysfConnectedList: Array.from({ length: 25 }, (_, i) => ({
        callsign: `CALL${i}`, ip: '192.0.2.1', type: 'Gateway', since: '10:00'
      })),
      echolinkAvailable: true,
      echolinkConnectedList: Array.from({ length: 26 }, (_, i) => ({
        number: `${i}`, user: `ECHO${i}`, attribute: 'T'
      }))
    });
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[aria-label="Paginación de estaciones YSF conectadas"]')).toBeNull();
    const echoPanel = root.querySelector('[aria-labelledby="echolink-title"]')!;
    expect(echoPanel.querySelectorAll('tbody tr')).toHaveLength(25);
    expect(echoPanel.textContent).toContain('Página 1 de 2');
    const next = Array.from(echoPanel.querySelectorAll('button')).find(button => button.textContent?.trim() === 'Siguiente')!;
    next.click();
    fixture.detectChanges();
    expect(echoPanel.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(echoPanel.textContent).toContain('ECHO25');

    const size = echoPanel.querySelector('select') as HTMLSelectElement;
    size.value = '5';
    size.dispatchEvent(new Event('change'));
    fixture.detectChanges();
    expect(echoPanel.textContent).toContain('Página 1 de 6');
    expect(echoPanel.querySelectorAll('tbody tr')).toHaveLength(5);

    const search = echoPanel.querySelector('input[type="search"]') as HTMLInputElement;
    search.value = 'echo2';
    search.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(echoPanel.textContent).toContain('Página 1 de 2');
    expect(echoPanel.querySelectorAll('tbody tr')).toHaveLength(5);
    expect(fixture.componentInstance.ysfConnectedPages.currentPage()).toBe(1);

    const service = TestBed.inject(ReflectorService);
    const data = service.data()!;
    service.data.set({ ...data, echolinkConnected: data.echolinkConnected.slice(0, 25) });
    fixture.detectChanges();
    expect(echoPanel.querySelector('nav')).toBeNull();
    expect(echoPanel.querySelector('input')).toBeNull();
    expect(echoPanel.querySelectorAll('tbody tr')).toHaveLength(25);
    expect(fixture.componentInstance.echolinkConnectedPages.search()).toBe('');
    fixture.destroy();
    http.verify();
  });

  it('labels every populated table cell for the mobile card layout', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      dvSwitchBridges: [{ modeA: 'DMR', modeB: 'YSF' }, { modeA: 'D-Star', modeB: 'NXDN' }],
      stats: { ysfConnectedNow: 1, ysfTx24h: 1, dmrTx24h: 1, dvSwitchTx24h: 6, stations24h: 1 },
      ysfConnectedList: [{ callsign: 'TEST1', ip: '192.0.2.1', type: 'Gateway', since: '10:00' }],
      ysfLastTx: [{ callsign: 'TEST1', duration: '5s', ago: '1m' }],
      dmrLastTx: [{ callsign: 'TEST1', duration: '5s', ago: '1m' }],
      dvSwitchLastTx: [
        { rxTime: '11-23 05:00:55', mode: 'P25', callsign: 'TEST1', name: '—', target: 'TG 740', source: 'Net', duration: '2.7', loss: '—', ber: '—' },
        { rxTime: '11-23 04:59:10', mode: 'YSF', callsign: 'TEST2', name: '—', target: 'TG 74001', source: 'LNet', duration: '—', loss: '—', ber: '—' },
        { rxTime: '11-23 04:58:59', mode: 'YSF', callsign: 'TEST3', name: '—', target: 'ALL at HC1LNM', source: 'Net', duration: '0.6', loss: '0%', ber: '0.0%' },
        { rxTime: '11-23 04:58:13', mode: 'NXDN', callsign: 'TEST4', name: '—', target: 'TG 8465', source: 'Net', duration: '2.6', loss: '—', ber: '0.0%' },
        { rxTime: '11-23 04:57:56', mode: 'D-Star', callsign: 'TEST5', name: '—', target: 'CQCQCQ', source: 'Net', duration: '2.5', loss: '0%', ber: '0.0%' },
        { rxTime: '11-23 04:56:25', mode: 'DMR', callsign: 'TEST6', name: '—', target: 'TG 368', source: 'LNet', duration: '2.7', loss: '—', ber: '—' }
      ],
      echolinkAvailable: true,
      echolinkConnectedList: [{ number: '1', user: 'TEST1', attribute: 'T' }],
      echolinkLastHeard: [{ callsign: 'TEST2', disconnectedAt: '2026-11-23T09:00:00-05:00', duration: '3m 10s' }]
    });
    fixture.detectChanges();
    const tables = fixture.nativeElement.querySelectorAll('table') as NodeListOf<HTMLTableElement>;
    expect(tables).toHaveLength(6);
    expect(Array.from(tables[0].querySelectorAll('thead th'), th => th.textContent?.trim()))
      .toEqual(['Indicativo', 'IP', 'Gateway', 'Última lectura']);
    expect(Array.from(tables[1].querySelectorAll('thead th'), th => th.textContent?.trim()))
      .toEqual(['Hace', 'Indicativo', 'Destino', 'Gateway', 'Duración', 'Pérdida']);
    expect(fixture.nativeElement.querySelector('.header-badges')).toBeNull();
    expect(Array.from(fixture.nativeElement.querySelectorAll('.bridge-badge') as NodeListOf<HTMLElement>,
      badge => badge.textContent?.trim())).toEqual([
      'Puente configurado: DMR ↔ YSF', 'Puente configurado: D-Star ↔ NXDN'
    ]);
    for (const table of tables) {
      const headings = Array.from(table.querySelectorAll('thead th'), th => th.textContent?.trim());
      const row = table.querySelector('tbody tr')!;
      const labels = Array.from(row.querySelectorAll('td, th'), cell => cell.getAttribute('data-label'));
      expect(labels).toEqual(headings);
    }
    const modeBadges = fixture.nativeElement.querySelectorAll('.dvswitch-table .mode-badge') as NodeListOf<HTMLElement>;
    expect(Array.from(modeBadges, badge => badge.textContent?.trim()))
      .toEqual(['P25', 'YSF', 'YSF', 'NXDN', 'D-Star', 'DMR']);
    expect(Array.from(modeBadges, badge => Array.from(badge.classList).find(name => name !== 'mode-badge')))
      .toEqual(['mode-p25', 'mode-ysf', 'mode-ysf', 'mode-nxdn', 'mode-d-star', 'mode-dmr']);
    expect(fixture.nativeElement.querySelector('.kpi-card.border-emerald .kpi-value').textContent.trim()).toBe('6');
    const dvSwitchPanel = fixture.nativeElement.querySelector('.dvswitch-table').closest('section') as HTMLElement;
    expect(dvSwitchPanel.querySelector('nav')).toBeNull();
    const search = dvSwitchPanel.querySelector('input[type="search"]') as HTMLInputElement;
    expect(search.closest('label')?.textContent).toContain('Filtrar en actividad DVSwitch');
    expect(search.closest('label')?.querySelector('svg')?.getAttribute('aria-hidden')).toBe('true');
    search.value = 'ysf';
    search.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(dvSwitchPanel.querySelectorAll('.dvswitch-table tbody tr')).toHaveLength(2);
    expect(dvSwitchPanel.textContent).toContain('TEST2');
    expect(dvSwitchPanel.textContent).not.toContain('TEST1');
    const service = TestBed.inject(ReflectorService);
    service.data.set({ ...service.data()!, dvSwitchTitle: 'Actividad personalizada' });
    fixture.detectChanges();
    expect(search.value).toBe('ysf');
    expect(dvSwitchPanel.querySelectorAll('.dvswitch-table tbody tr')).toHaveLength(2);
    const dvSwitchSection = fixture.nativeElement.querySelector('.dvswitch-section') as HTMLElement;
    expect(dvSwitchSection.querySelector('summary')?.textContent?.trim()).toBe('DVSwitch');
    expect(dvSwitchSection.querySelector('.panel-header h2')?.textContent?.trim()).toBe('Actividad personalizada');
    search.value = 'no existe';
    search.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(dvSwitchPanel.textContent).toContain('Sin resultados para la búsqueda');
    fixture.destroy();
    http.verify();
  });

  it('displays the identity of each reflector in its collapsible section', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 },
      ysfReflectorInfo: { available: true, id: '74002', name: 'EC ECUADOR', description: 'C4FM', port: '42000' },
      pysfReflector: { available: true, id: '74001', name: 'EC QUITO', description: 'YSF ROOM C4FM', port: '42001', connected: [], lastHeard: [] }
    });
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    const ysf = root.querySelector('.ysf-section') as HTMLDetailsElement;
    const pysf = root.querySelector('.pysf-section') as HTMLDetailsElement;
    expect(ysf.querySelector('summary')?.textContent).toContain('EC ECUADOR');
    expect(ysf.querySelector('summary')?.textContent).toContain('74002');
    expect(ysf.querySelector('summary')?.textContent).toContain('C4FM');
    expect(ysf.querySelector('summary')?.textContent).toContain('42000');
    expect(pysf.querySelector('summary')?.textContent).toContain('EC QUITO');
    expect(pysf.querySelector('summary')?.textContent).toContain('74001');
    expect(pysf.querySelector('summary')?.textContent).toContain('YSF ROOM C4FM');
    expect(pysf.querySelector('summary')?.textContent).toContain('42001');
    expect(ysf.open).toBe(true);
    ysf.open = false;
    fixture.detectChanges();
    expect(ysf.open).toBe(false);
    expect(pysf.open).toBe(true);
    fixture.destroy();
    http.verify();
  });

  it('displays the IP of each connected YSF station', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    http.expectOne((request) => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne((request) => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 2, ysfTx24h: 0, dmrTx24h: 0, stations24h: 1 },
      ysfConnectedList: [
        { callsign: 'TEST1', ip: '192.0.2.1', type: 'Gateway', since: '10:00' },
        { callsign: 'TEST1', ip: '192.0.2.2', type: 'Gateway', since: '10:00' }
      ]
    });
    fixture.detectChanges();
    const table = fixture.nativeElement.querySelector('table') as HTMLTableElement;
    expect(table.textContent).toContain('IP');
    expect(table.textContent).toContain('192.0.2.1');
    expect(table.textContent).toContain('192.0.2.2');
    expect(table.querySelectorAll('tbody tr')).toHaveLength(2);
    fixture.destroy();
    http.verify();
  });

  it('displays a connection error', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    http.expectOne((request) => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne((request) => request.url.endsWith('/api/reflector-status'))
      .flush('error', { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[role="alert"]').textContent)
      .toContain('Sin conexión');
    fixture.destroy();
    http.verify();
  });

  it('keeps the local dashboard selectable when a remote site fails', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    const status = {
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
    };
    http.expectOne((request) => request.url.endsWith('/api/profiles'))
      .flush([{ id: 'remote', name: 'Remoto', visible: true }]);
    http.expectOne((request) => request.url.endsWith('/api/reflector-status')).flush(status);
    fixture.detectChanges();
    const select = fixture.nativeElement.querySelector('#site') as HTMLSelectElement;
    select.value = 'remote';
    select.dispatchEvent(new Event('change'));
    http.expectOne((request) => request.url.endsWith('/api/profiles/remote/status'))
      .flush('error', { status: 502, statusText: 'Bad Gateway' });
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Sin conexión');
    expect(fixture.nativeElement.textContent).not.toContain('Local');
    select.value = '';
    select.dispatchEvent(new Event('change'));
    http.expectOne((request) => request.url.endsWith('/api/reflector-status')).flush(status);
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Local');
    fixture.destroy();
    http.verify();
  });

  it('refreshes the selected site manually, including after an error', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    http.expectOne(request => request.url.endsWith('/api/profiles'))
      .flush([{ id: 'remote', name: 'Remoto', visible: true }]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status'))
      .flush('error', { status: 500, statusText: 'Error' });
    const select = fixture.nativeElement.querySelector('#site') as HTMLSelectElement;
    fixture.detectChanges();
    select.value = 'remote';
    select.dispatchEvent(new Event('change'));
    http.expectOne(request => request.url.endsWith('/api/profiles/remote/status'))
      .flush('error', { status: 502, statusText: 'Error' });
    fixture.detectChanges();
    const button = Array.from(fixture.nativeElement.querySelectorAll('button') as NodeListOf<HTMLButtonElement>)
      .find(element => element.textContent?.includes('Refrescar datos'));
    expect(button).toBeDefined();
    button!.click();
    http.expectOne(request => request.url.endsWith('/api/profiles/remote/status')).flush({
      header: { title: 'Remoto', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Remoto');
    expect(TestBed.inject(ReflectorService).error()).toBeNull();
    fixture.destroy();
    http.verify();
  });

  it('hides EchoLink tables when live EchoLink is unavailable, even with saved history', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 },
      echolinkAvailable: false,
      echolinkLastHeard: [{ callsign: 'KQ4HPI-R', disconnectedAt: '2026-11-23T09:00:00-05:00', duration: '3m 10s' }]
    });
    fixture.detectChanges();

    expect(fixture.nativeElement.querySelector('#echolink-title')).toBeNull();
    expect(fixture.nativeElement.querySelector('#echolink-history-title')).toBeNull();
    fixture.destroy();
    http.verify();
  });

  it('hides all source cards and tables without alerts when sources are absent', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 },
      ysfAvailable: false,
      ysfTrafficAvailable: false,
      dmrAvailable: false,
      echolinkAvailable: false,
      echolinkLastHeard: [],
      warnings: ['No se encontraron logs YSF recientes', 'No se encontraron logs DMR recientes', 'EchoLink no está disponible']
    });
    fixture.detectChanges();

    expect(fixture.nativeElement.querySelector('[role="alert"]')).toBeNull();
    expect(fixture.nativeElement.querySelector('.dash-header h1').textContent).toContain('Local');
    expect(fixture.nativeElement.querySelector('.kpi-grid')).toBeNull();
    expect(fixture.nativeElement.querySelectorAll('table')).toHaveLength(0);
    expect(fixture.nativeElement.querySelector('.bridge-badge')).toBeNull();
    fixture.destroy();
    http.verify();
  });

  it('shows only cards and tables supported by available sources', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    const status = {
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 1, ysfTx24h: 2, dmrTx24h: 3, stations24h: 4 }
    };
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      ...status, ysfAvailable: false, ysfTrafficAvailable: true, dmrAvailable: true, echolinkAvailable: false
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.border-cyan')).toBeNull();
    expect(fixture.nativeElement.querySelector('.border-purple')).toBeNull();
    expect(fixture.nativeElement.querySelector('.border-blue')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.border-emerald')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.ysf-last-heard')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.dvswitch-table')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('#echolink-title')).toBeNull();

    TestBed.inject(ReflectorService).fetchStatusOnce().subscribe();
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      ...status, ysfAvailable: true, ysfTrafficAvailable: true, dmrAvailable: false, echolinkAvailable: true
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.border-cyan')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.border-blue')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.border-emerald')).toBeNull();
    expect(fixture.nativeElement.querySelector('.border-purple')).toBeNull();
    expect(fixture.nativeElement.querySelector('.dvswitch-table')).toBeNull();
    expect(fixture.nativeElement.querySelector('#echolink-title')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('#echolink-history-title')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('[role="alert"]')).toBeNull();
    fixture.destroy();
    http.verify();
  });

  it('shows direct EchoLink action failures in the table panel', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 },
      echolinkAvailable: true,
      echolinkConnectedList: [{ number: '1', user: 'TEST1', attribute: '' }]
    });
    fixture.detectChanges();
    const panel = fixture.nativeElement.querySelector('[aria-labelledby="echolink-title"]') as HTMLElement;
    (panel.querySelector('[aria-label="Mutear estación"]') as HTMLButtonElement).click();
    http.expectOne(request => request.method === 'POST' && request.url.endsWith('/api/admin/echolink/action'))
      .flush({ message: 'No se pudo ejecutar tlbcmd' }, { status: 400, statusText: 'Bad Request' });
    fixture.detectChanges();
    expect(panel.querySelector('[role="alert"]')?.textContent).toContain('No se pudo ejecutar tlbcmd');
    fixture.destroy();
    http.verify();
  });

    it('offers global actions, ban management and distinct mute states', async () => {
      await TestBed.configureTestingModule({
        imports: [ReflectorDashboardComponent],
        providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
      }).compileComponents();
      const fixture = TestBed.createComponent(ReflectorDashboardComponent);
      fixture.detectChanges();
      const http = TestBed.inject(HttpTestingController);
      http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush([]);
      http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
      const status = {
        header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
        stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 },
        echolinkAvailable: true,
        echolinkConnectedList: [
          { number: '1', user: 'TEST1', attribute: '' },
          { number: '2', user: 'TEST2', attribute: 'IMKx' }
        ]
      };
      http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush(status);
      fixture.detectChanges();
      const root = fixture.nativeElement as HTMLElement;
      const panel = root.querySelector('[aria-labelledby="echolink-title"]')!;
      expect(panel.querySelector('[aria-label="Mutear estación"]')?.classList.contains('mute-inactive')).toBe(true);
      expect(panel.querySelector('[aria-label="Desmutear estación"]')?.classList.contains('mute-active')).toBe(true);
      const inactiveStation = Array.from(panel.querySelectorAll('tbody tr'))
        .find(row => row.textContent?.includes('TEST2'));
      expect(inactiveStation?.classList.contains('echolink-inactive-conference')).toBe(true);
      expect(panel.querySelectorAll('[aria-label="Banear estación"]')).toHaveLength(2);
      const dialog = root.querySelector('#echolink-action-title')!.closest('dialog') as HTMLDialogElement;
      dialog.showModal = vi.fn();
      dialog.close = vi.fn();
      const globalMenu = panel.querySelector('.admin-menu')!;
      const buttons = Array.from(globalMenu.querySelectorAll('button'));
      expect(buttons.map(button => button.textContent?.trim())).toEqual(['Conectar', 'Mensaje', 'Lista de baneados', 'Reintentos pendientes']);
      (globalMenu as HTMLDetailsElement).open = true;
      globalMenu.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      expect((globalMenu as HTMLDetailsElement).open).toBe(true);
      document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      expect((globalMenu as HTMLDetailsElement).open).toBe(false);
      buttons[2].click();
      const listRequest = http.expectOne(request => request.url.endsWith('/api/admin/echolink/action'));
      expect(listRequest.request.body).toEqual({ action: 'ban-list' });
      listRequest.flush({ output: 'tlbcmd Version 0.56\n0\nbanned users:\nW1AW -\nKQ4ABC host.example\n' });
      fixture.detectChanges();
      expect(dialog.querySelectorAll('.banned-stations li')).toHaveLength(2);
      (dialog.querySelector('.banned-stations button') as HTMLButtonElement).click();
      const unban = http.expectOne(request => request.url.endsWith('/api/admin/echolink/action'));
      expect(unban.request.body).toEqual({ action: 'unban', callsign: 'W1AW' });
      unban.flush({ output: 'W1AW removed from the banned list.' });
      http.expectOne(request => request.url.endsWith('/api/admin/echolink/action'))
        .flush({ output: '0\nbanned users:\nKQ4ABC host.example\n' });
      http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush(status);
      fixture.detectChanges();
      expect(dialog.querySelector('.banned-stations')?.textContent).not.toContain('W1AW');
      expect(dialog.querySelector('.banned-stations')?.textContent).toContain('KQ4ABC');

      expect(buttons[0].classList.contains('btn-dark')).toBe(true);
      expect(buttons[0].dataset['icon']).toBeTruthy();
      buttons[0].click();
      fixture.detectChanges();
      const callsignInput = dialog.querySelector('label input:not([type])') as HTMLInputElement;
      callsignInput.value = 'W1AW';
      callsignInput.dispatchEvent(new Event('input'));
      const permanent = dialog.querySelector('input[type="checkbox"]') as HTMLInputElement;
      permanent.checked = true;
      permanent.dispatchEvent(new Event('change'));
      fixture.detectChanges();
      (Array.from(dialog.querySelectorAll('button')).find(button => button.textContent?.trim() === 'Aplicar') as HTMLButtonElement).click();
      const connect = http.expectOne(request => request.url.endsWith('/api/admin/echolink/action'));
      expect(connect.request.body).toEqual({ action: 'connect', callsign: 'W1AW', permanent: true });
      fixture.detectChanges();
      expect(dialog.close).toHaveBeenCalled();
      const connecting = panel.querySelector('tr.echolink-connecting')!;
      expect(connecting.textContent).toContain('W1AW');
      expect(connecting.textContent).toContain('Conectando');
      expect(connecting.getAttribute('aria-busy')).toBe('true');
      connect.flush({ output: 'No se pudo conectar con W1AW; se reintentará cada 12 s en segundo plano.' });
      http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush(status);
      fixture.detectChanges();
      expect(panel.querySelector('tr.echolink-connecting')?.textContent).toContain('W1AW');
      expect(panel.textContent).toContain('se reintentará');

      fixture.componentInstance.refresh();
      http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
        ...status,
        echolinkConnectedList: [...status.echolinkConnectedList, { number: '3', user: 'W1AW', attribute: 'P' }]
      });
      fixture.detectChanges();
      expect(panel.querySelector('tr.echolink-connecting')).toBeNull();

      (panel.querySelector('[aria-label="Desconectar estación"]') as HTMLButtonElement).click();
      const disconnect = http.expectOne(request => request.url.endsWith('/api/admin/echolink/action'));
      fixture.detectChanges();
      const disconnecting = panel.querySelector('tr.echolink-disconnecting')!;
      expect(disconnecting.textContent).toContain('TEST1');
      expect(disconnecting.textContent).toContain('Desconectando');
      expect(disconnecting.getAttribute('aria-busy')).toBe('true');
      disconnect.flush({ message: 'Error al desconectar' }, { status: 400, statusText: 'Bad Request' });
      fixture.detectChanges();
      expect(panel.querySelector('tr.echolink-disconnecting')).toBeNull();
      expect(panel.querySelector('[role="alert"]')?.textContent).toContain('Error al desconectar');
      (panel.querySelector('[aria-label="Desconectar estación"]') as HTMLButtonElement).click();
      http.expectOne(request => request.url.endsWith('/api/admin/echolink/action'))
        .flush({ output: 'tlbcmd Version 0.56 0 TEST1 disconnected' });
      http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush(status);
      fixture.detectChanges();
      expect(panel.textContent).not.toContain('tlbcmd Version');

      buttons[0].click();
      fixture.componentInstance.actionDialogInput.set('test1');
      fixture.componentInstance.submitEchoLinkAction(dialog);
      const reconnect = http.expectOne(request => request.url.endsWith('/api/admin/echolink/action'));
      fixture.detectChanges();
      expect(panel.querySelectorAll('tr.echolink-connecting')).toHaveLength(1);
      expect(panel.querySelector('tr.echolink-connecting')?.textContent).toContain('TEST1');
      reconnect.flush({ message: 'Error al conectar' }, { status: 400, statusText: 'Bad Request' });
      fixture.detectChanges();
      expect(panel.querySelector('tr.echolink-connecting')).toBeNull();
      expect(panel.querySelector('[role="alert"]')?.textContent).toContain('Error al conectar');
      fixture.destroy();
      http.verify();
    });

    it('hides confirmation while the setup password is visible and restores its validation', async () => {
      await TestBed.configureTestingModule({
        imports: [ReflectorDashboardComponent],
        providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
      }).compileComponents();
      const fixture = TestBed.createComponent(ReflectorDashboardComponent);
      fixture.detectChanges();
      const http = TestBed.inject(HttpTestingController);
      http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
      http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
      http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
        header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
        stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
      });
      fixture.detectChanges();
      const root = fixture.nativeElement as HTMLElement;
      const dialog = root.querySelector('#admin-dialog-title')!.closest('dialog') as HTMLDialogElement;
      dialog.showModal = vi.fn();
      dialog.close = vi.fn();
      (Array.from(root.querySelectorAll('button')).find(button => button.textContent?.trim() === 'Admin')!).click();
      http.expectOne(request => request.url.endsWith('/api/admin/setup')).flush({ required: true });
      fixture.detectChanges();
      fixture.componentInstance.adminForm.patchValue({ password: 'password123', confirm: 'wrong' });
      (dialog.querySelector('form') as HTMLFormElement).dispatchEvent(new Event('submit'));
      fixture.detectChanges();
      expect(dialog.textContent).toContain('coincidir con la confirmación');
      (dialog.querySelector('[aria-label="Mostrar contraseña"]') as HTMLButtonElement).click();
      fixture.detectChanges();
      expect(dialog.querySelector('#dashboard-admin-confirm')).toBeNull();
      (dialog.querySelector('[aria-label="Ocultar contraseña"]') as HTMLButtonElement).click();
      fixture.detectChanges();
      expect(dialog.querySelector('#dashboard-admin-confirm')).not.toBeNull();
      (dialog.querySelector('form') as HTMLFormElement).dispatchEvent(new Event('submit'));
      fixture.detectChanges();
      expect(dialog.textContent).toContain('coincidir con la confirmación');
      (dialog.querySelector('[aria-label="Mostrar contraseña"]') as HTMLButtonElement).click();
      fixture.detectChanges();
      fixture.componentInstance.adminForm.patchValue({ confirm: '' });
      (dialog.querySelector('form') as HTMLFormElement).dispatchEvent(new Event('submit'));
      const setup = http.expectOne(request => request.url.endsWith('/api/admin/setup') && request.method === 'POST');
      expect(setup.request.body).toEqual({ password: 'password123' });
      setup.flush(null, { status: 204, statusText: 'No Content' });
      http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
      fixture.destroy();
      http.verify();
    });

    it('opens the administrator login dialog and updates the menu after login', async () => {
      await TestBed.configureTestingModule({
        imports: [ReflectorDashboardComponent],
        providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
      }).compileComponents();
      const fixture = TestBed.createComponent(ReflectorDashboardComponent);
      fixture.detectChanges();
      const http = TestBed.inject(HttpTestingController);
      http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
      http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
      http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
        header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
        stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
      });
      fixture.detectChanges();
      const root = fixture.nativeElement as HTMLElement;
      const dialog = root.querySelector('#admin-dialog-title')!.closest('dialog') as HTMLDialogElement;
      dialog.showModal = vi.fn();
      dialog.close = vi.fn();
      (Array.from(root.querySelectorAll('button')).find(button => button.textContent?.trim() === 'Admin')!).click();
      http.expectOne(request => request.url.endsWith('/api/admin/setup')).flush({ required: false });
      fixture.detectChanges();
      expect(dialog.querySelector('.login-logo')?.getAttribute('src')).toBe('ameriboard-icon.png');
      const passwordInput = dialog.querySelector('#dashboard-admin-password') as HTMLInputElement;
      expect(passwordInput.type).toBe('password');
      (dialog.querySelector('[aria-label="Mostrar contraseña"]') as HTMLButtonElement).click();
      fixture.detectChanges();
      expect(passwordInput.type).toBe('text');
      fixture.componentInstance.adminForm.patchValue({ password: 'password123' });
      (dialog.querySelector('form') as HTMLFormElement).dispatchEvent(new Event('submit'));
      const login = http.expectOne(request => request.url.endsWith('/api/admin/login'));
      expect(login.request.body).toEqual({ password: 'password123' });
      login.flush(null, { status: 204, statusText: 'No Content' });
      http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
      fixture.detectChanges();
      expect(root.querySelector('.site-selector .admin-menu')?.textContent).toContain('Administrar sitios');
      expect(fixture.componentInstance.adminForm.value.password).toBe('');
      (Array.from(root.querySelectorAll('button')).find(button => button.textContent?.trim() === 'Cerrar sesión')!).click();
      http.expectOne(request => request.url.endsWith('/api/admin/logout')).flush(null, { status: 204, statusText: 'No Content' });
      http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
        header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
        stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
      });
      http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
      fixture.detectChanges();
      expect(root.querySelector('.site-selector .admin-menu')).toBeNull();
      fixture.destroy();
      http.verify();
    });
  it('hides unavailable EchoLink and shows its attribute legend when available', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush(null, { status: 401, statusText: 'Unauthorized' });
    const status = {
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
    };
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status'))
      .flush({ ...status, echolinkAvailable: false });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#echolink-title')).toBeNull();

    TestBed.inject(ReflectorService).fetchStatusOnce().subscribe();
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      ...status, echolinkAvailable: true,
      echolinkConnectedList: [{ number: '1', user: 'TEST', attribute: 'T' }]
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#echolink-title').textContent).toContain('EchoLink');
    const echoPanels = fixture.nativeElement.querySelectorAll('.echolink-grid > .panel-card');
    expect(echoPanels).toHaveLength(2);
    expect(echoPanels[0].querySelector('#echolink-title')).not.toBeNull();
    expect(echoPanels[1].querySelector('#echolink-history-title')).not.toBeNull();
    expect(echoPanels[1].textContent).toContain('Aún no hay desconexiones registradas.');
    const dialog = fixture.nativeElement.querySelector('.echolink-legend') as HTMLDialogElement;
    const show = vi.fn();
    const close = vi.fn();
    dialog.showModal = show;
    dialog.close = close;
    fixture.nativeElement.querySelector('.echolink-actions [aria-haspopup="dialog"]').click();
    expect(show).toHaveBeenCalledOnce();
    expect(Array.from(dialog.querySelectorAll('tbody th')).map(cell => cell.textContent?.trim()))
      .toEqual(['A', 'a', 'B', 'C', 'c', 'F', 'f', 'K', 'L', 'M', 'm', 'P', 'R', 'S', 'T', 'u', 'x', '*', '0', '1', '!']);
    dialog.querySelector('button')?.click();
    expect(close).toHaveBeenCalledOnce();
    fixture.destroy();
    http.verify();
  });
});

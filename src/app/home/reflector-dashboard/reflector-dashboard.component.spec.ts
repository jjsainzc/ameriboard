import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { ReflectorDashboardComponent } from './reflector-dashboard.component';

describe('ReflectorDashboardComponent', () => {
  it('displays the IP of each connected YSF station', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
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
      providers: [provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne((request) => request.url.endsWith('/api/reflector-status'))
      .flush('error', { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[role="alert"]').textContent)
      .toContain('Sin conexión');
    fixture.destroy();
    http.verify();
  });
});

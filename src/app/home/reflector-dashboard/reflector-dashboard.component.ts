import { DatePipe } from '@angular/common';
import { Component, computed, DestroyRef, inject, OnInit, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { RouterLink } from '@angular/router';
import { ReflectorService, SiteProfile } from '../../core/services/reflector.service';
import { TablePagination, TablePaginationComponent } from '../../shared/table-pagination/table-pagination.component';

@Component({
  selector: 'app-reflector-dashboard',
  imports: [RouterLink, DatePipe, TablePaginationComponent],
  templateUrl: './reflector-dashboard.component.html',
  styleUrl: './reflector-dashboard.components.scss'
})
export class ReflectorDashboardComponent implements OnInit {
  private readonly reflectorService = inject(ReflectorService);
  private readonly destroyRef = inject(DestroyRef);

  readonly data = this.reflectorService.data;
  readonly error = this.reflectorService.error;
  readonly loading = this.reflectorService.loading;
  readonly profiles = signal<SiteProfile[]>([]);
  readonly profilesError = signal('');
  readonly ysfConnectedPages = new TablePagination(computed(() => this.data()?.ysfConnected ?? []));
  readonly ysfLastHeardPages = new TablePagination(computed(() => this.data()?.ysfLastHeard ?? []));
  readonly echolinkConnectedPages = new TablePagination(computed(() => this.data()?.echolinkConnected ?? []));
  readonly echolinkLastHeardPages = new TablePagination(computed(() => this.data()?.echolinkLastHeard ?? []), station => [
    station.callsign, station.duration, station.disconnectedAt,
    new DatePipe('en-US').transform(station.disconnectedAt, 'dd/MM/yyyy HH:mm:ss') ?? ''
  ]);
  readonly dvSwitchPages = new TablePagination(computed(() => this.data()?.dvSwitchLastHeard ?? []), undefined, true);
  readonly echolinkAttributes = [
    { code: 'A', meaning: 'Sesión iniciada como administrador.' },
    { code: 'a', meaning: 'Códec ADPCM.' },
    { code: 'B', meaning: 'Conferencia theBridge enlazada.' },
    { code: 'C', meaning: 'Conferencia enlazada distinta de theBridge.' },
    { code: 'c', meaning: 'Texto del chat suprimido.' },
    { code: 'F', meaning: 'Reproduciendo un archivo mediante .play o .test.' },
    { code: 'f', meaning: 'Conexión full dúplex.' },
    { code: 'K', meaning: 'Expulsado.' },
    { code: 'L', meaning: 'Oyente pasivo.' },
    { code: 'M', meaning: 'Audio y texto silenciados.' },
    { code: 'm', meaning: 'Audio silenciado.' },
    { code: 'P', meaning: 'Conexión permanente mediante .connect.' },
    { code: 'R', meaning: 'Solo recepción (monitoreado).' },
    { code: 'S', meaning: 'Sesión iniciada como operador del sistema (sysop).' },
    { code: 'T', meaning: 'Transmitiendo en este momento.' },
    { code: 'u', meaning: 'Códec uLaw.' },
    { code: 'x', meaning: 'Actualmente inactivo en la conferencia.' },
    { code: '*', meaning: 'Usa el protocolo Asterisk.' },
    { code: '0', meaning: 'Usa el protocolo Speak Freely.' },
    { code: '1', meaning: 'Usa el protocolo RTP.' },
    { code: '!', meaning: 'Versión antigua de theBridge que envía paquetes SDES con una extensión privada "txt".' }
  ];

  ngOnInit(): void {
    this.reflectorService.selectProfile('');
    this.reflectorService.pollStatus(5000).pipe(takeUntilDestroyed(this.destroyRef)).subscribe();
    this.reflectorService.getProfiles().pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: profiles => this.profiles.set(profiles),
      error: () => this.profilesError.set('No se pudo cargar la lista de sitios; el dashboard local sigue disponible.')
    });
  }

  selectProfile(event: Event): void {
    for (const pagination of [this.ysfConnectedPages, this.ysfLastHeardPages, this.echolinkConnectedPages,
      this.echolinkLastHeardPages, this.dvSwitchPages]) {
      pagination.reset();
    }
    this.reflectorService.selectProfile((event.target as HTMLSelectElement).value);
  }

  refresh(): void {
    this.reflectorService.refresh();
  }
}

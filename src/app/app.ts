import { DOCUMENT, NgOptimizedImage } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { RouterOutlet } from '@angular/router';

@Component({
  imports: [RouterOutlet, NgOptimizedImage],
  selector: 'app-root',
  styleUrl: './app.scss',
  templateUrl: './app.html',
})
export class App {
  private readonly http = inject(HttpClient);
  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly year = new Date().getFullYear();
  protected readonly version = signal('consultando...');

  constructor() {
    const base = (this.document.querySelector('base')?.getAttribute('href') ?? '/').replace(/\/$/, '');
    this.http.get<{ version: string }>(`${base}/api/version`)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: result => this.version.set(result.version || 'no disponible'),
        error: error => {
          console.error('No se pudo consultar la versión de Ameriboard', error);
          this.version.set('no disponible');
        }
      });
  }
}

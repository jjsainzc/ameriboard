import { Component, DestroyRef, inject, OnInit } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ReflectorService } from '../../core/services/reflector.service';

@Component({
  selector: 'app-reflector-dashboard',
  templateUrl: './reflector-dashboard.component.html',
  styleUrl: './reflector-dashboard.components.scss'
})
export class ReflectorDashboardComponent implements OnInit {
  private readonly reflectorService = inject(ReflectorService);
  private readonly destroyRef = inject(DestroyRef);

  readonly data = this.reflectorService.data;
  readonly error = this.reflectorService.error;
  readonly loading = this.reflectorService.loading;

  ngOnInit(): void {
    this.reflectorService.pollStatus(5000).pipe(takeUntilDestroyed(this.destroyRef)).subscribe();
  }
}

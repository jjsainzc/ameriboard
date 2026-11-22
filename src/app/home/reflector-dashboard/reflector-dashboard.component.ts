import { Component, inject, OnInit } from '@angular/core';
import { ReflectorService } from '../../core/services/reflector.service';

@Component({
  selector: 'app-reflector-dashboard',
  standalone: true,
  templateUrl: './reflector-dashboard.component.html',
  styleUrl: './reflector-dashboard.component.scss'
})
export class ReflectorDashboardComponent implements OnInit {
  private readonly reflectorService = inject(ReflectorService);

  readonly data = this.reflectorService.data;
  readonly error = this.reflectorService.error;
  readonly loading = this.reflectorService.loading;

  ngOnInit(): void {
    this.reflectorService.pollStatus(5000).subscribe();
  }
}

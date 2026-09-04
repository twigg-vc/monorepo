// Helper timer to ensure functions have a minimum duration.
// Useful, for example, to make callbacks take at least x time for a loader
// to appear.
export class MinDurationTimer {
    private start: number;
    private readonly minMs: number;
  
    constructor(minMs: number = 300) {
      this.start = performance.now();
      this.minMs = minMs;
    }
    
    // If needed, wait until `minMs` have passed since the construction
    async Wait(): Promise<void> {
      const elapsed = performance.now() - this.start;
      const remaining = this.minMs - elapsed;
      if (remaining > 0) {
        await new Promise(resolve => setTimeout(resolve, remaining));
      }
    }
  }
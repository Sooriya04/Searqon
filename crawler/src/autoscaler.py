import asyncio
import logging
import psutil
from typing import Callable, Coroutine, Any, Set

logger = logging.getLogger(__name__)

class Autoscaler:
    """
    Intelligently manages the number of concurrent tasks based 
    on system CPU/Memory to maximize throughput safely.
    """
    def __init__(self, 
                 run_task_func: Callable[[], Coroutine[Any, Any, None]],
                 min_concurrency: int = 1,
                 max_concurrency: int = 50,
                 scale_up_step: int = 1,
                 scale_down_step: int = 1,
                 cpu_limit_pct: float = 80.0,
                 memory_limit_pct: float = 80.0):
        self.run_task_func = run_task_func
        self.min_concurrency = min_concurrency
        self.max_concurrency = max_concurrency
        self.scale_up_step = scale_up_step
        self.scale_down_step = scale_down_step
        self.cpu_limit_pct = cpu_limit_pct
        self.memory_limit_pct = memory_limit_pct
        
        self.current_concurrency = min_concurrency
        self._workers: Set[asyncio.Task] = set()
        self._running = False
        self._manager_task: asyncio.Task | None = None

    async def start(self) -> None:
        """Starts the autoscaling background monitor."""
        if self._running:
            return
        self._running = True
        logger.info("Autoscaler started.")
        self._manager_task = asyncio.create_task(self._manage_pool())

    async def stop(self) -> None:
        """Stops the autoscaler and awaits all underlying task completion."""
        self._running = False
        if self._manager_task:
            self._manager_task.cancel()
            try:
                await self._manager_task
            except asyncio.CancelledError:
                pass
            
        if self._workers:
            # Wait for any active tasks to finish cleanly
            await asyncio.gather(*self._workers, return_exceptions=True)
            self._workers.clear()
        logger.info("Autoscaler stopped cleanly.")

    def _is_system_overloaded(self) -> bool:
        """Checks if current system utilization exceeds thresholds."""
        cpu = psutil.cpu_percent(interval=None)
        memory = psutil.virtual_memory().percent
        return cpu > self.cpu_limit_pct or memory > self.memory_limit_pct

    async def _manage_pool(self) -> None:
        """Evaluates periodically to scale up or down workers."""
        while self._running:
            # 1. Clean up done tasks
            done_tasks = [t for t in self._workers if t.done()]
            for t in done_tasks:
                self._workers.remove(t)
                # Ensure we surface any exceptions
                if t.exception():
                    logger.error(f"Worker task raised an exception: {t.exception()}")

            # 2. Evaluate limits (Snapshot system state)
            overloaded = self._is_system_overloaded()
            
            if overloaded and self.current_concurrency > self.min_concurrency:
                self.current_concurrency = max(self.min_concurrency, self.current_concurrency - self.scale_down_step)
                logger.debug(f"System overloaded. Scaling DOWN to {self.current_concurrency}.")
            elif not overloaded and self.current_concurrency < self.max_concurrency:
                self.current_concurrency = min(self.max_concurrency, self.current_concurrency + self.scale_up_step)
                logger.debug(f"System healthy. Scaling UP to {self.current_concurrency}.")

            # 3. Span missing workers to match current_concurrency
            while len(self._workers) < self.current_concurrency:
                # We must wrap the coroutine function to ensure it doesn't just
                # return a coroutine object but actually runs it in a loop
                # while the autoscaler considers it active.
                async def worker_wrapper():
                    while self._running:
                        try:
                            await self.run_task_func()
                        except asyncio.CancelledError:
                            break
                        except Exception as e:
                            logger.error(f"Worker task error: {e}")
                    
                task = asyncio.create_task(worker_wrapper())
                self._workers.add(task)
                
            # Check every second (adjustable snapshotting interval)
            await asyncio.sleep(1)

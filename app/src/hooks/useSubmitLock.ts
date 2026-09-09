import { useCallback, useRef, useState } from 'react';

export function useSubmitLock() {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const isSubmittingRef = useRef(false);

  const run = useCallback(async (submit: () => Promise<void>) => {
    if (isSubmittingRef.current) return;
    isSubmittingRef.current = true;
    setIsSubmitting(true);
    try {
      await submit();
    } finally {
      isSubmittingRef.current = false;
      setIsSubmitting(false);
    }
  }, []);

  return { isSubmitting, run };
}

import type {ReactNode} from 'react';
import clsx from 'clsx';

import styles from './styles.module.css';

type Props = {
  className?: string;
};

/**
 * The "Tailrelay" wordmark, matching the Web UI's navbar and login masthead:
 * a solid "Tail" followed by an outlined "relay".
 *
 * Kept as one component so the docs site and the app can't drift apart. See
 * `.brand-relay` in webui/frontend/src/app.css for the original rule.
 */
export default function BrandName({className}: Props): ReactNode {
  return (
    <span className={clsx(styles.brand, className)}>
      Tail<span className={styles.relay}>relay</span>
    </span>
  );
}

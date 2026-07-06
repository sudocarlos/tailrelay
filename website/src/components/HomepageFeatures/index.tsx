import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Browser-Based Management',
    description: (
      <>
        A responsive Web UI on port <code>8021</code> to configure HTTPS and
        TCP relays, Funnel, backups, and live logs — no CLI required.
      </>
    ),
  },
  {
    title: 'Automatic TLS via Tailscale Serve',
    description: (
      <>
        HTTPS relays get automatic MagicDNS hostnames and TLS termination
        courtesy of <code>tailscale serve</code>. TCP relays forward
        non-HTTP protocols the same way.
      </>
    ),
  },
  {
    title: 'Funnel for Public Access',
    description: (
      <>
        Expose a service to the public internet on port <code>443</code>,{' '}
        <code>8443</code>, or <code>10000</code> via Tailscale Funnel, with
        dual authentication (Tailscale identity or token) protecting the
        Web UI itself.
      </>
    ),
  },
];

function Feature({title, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

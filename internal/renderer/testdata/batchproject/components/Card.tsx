import { shout } from '@/lib/format';

export default function Card({ title }: { title: string }) {
  return <div>{shout(title)}</div>;
}

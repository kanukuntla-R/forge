import missing from 'this-package-does-not-exist';

export default function Broken() {
  return <div>{missing}</div>;
}

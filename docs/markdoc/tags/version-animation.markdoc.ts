import { VersionAnimation } from '../../components/VersionAnimation';

export const versionAnimation = {
  render: VersionAnimation,
  description: 'Display an animated semantic version counter with deployment logs',
  attributes: {
    className: { type: String },
  },
  selfClosing: true,
};

import {Callout} from '../../components';

export const callout = {
  render: Callout,
  children: ['paragraph', 'tag', 'list'],
  attributes: {
    type: {
      type: String,
      default: 'note',
      matches: ['caution', 'check', 'note', 'warning'],
      errorLevel: 'critical'
    },
    title: {
      type: String,
    },
  },
};
